package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

const diagnosticReportSchema = "ray.diagnostic-report/v1"
const maxDiagnosticOutput = 16 << 20

// diagnosticsEvidence binds supporting diagnostic executions. These records
// can block the production path, but they are never interpreted as one of the
// four formal results and therefore cannot manufacture VERIFIED.
type diagnosticsEvidence struct {
	SpecIRDigest string                       `json:"spec_ir_digest"`
	PICT         pictDiagnosticEvidence       `json:"pict"`
	Oracle       commandDiagnosticEvidence    `json:"oracle"`
	DiffTest     commandDiagnosticEvidence    `json:"diff_test"`
	Dependency   dependencyDiagnosticEvidence `json:"dependency"`
	Complete     bool                         `json:"complete"`
}

type pictDiagnosticEvidence struct {
	Tool         semanticir.ToolRef `json:"tool"`
	Strength     int                `json:"strength"`
	ModelDigest  string             `json:"model_digest"`
	Argv         []string           `json:"argv"`
	StdoutDigest string             `json:"stdout_digest"`
	StderrDigest string             `json:"stderr_digest"`
	Combinations int                `json:"combinations"`
	Complete     bool               `json:"complete"`
}

type commandDiagnosticEvidence struct {
	Kind              string                           `json:"kind"`
	Tool              semanticir.ToolRef               `json:"tool"`
	Subject           semanticir.ArtifactRef           `json:"subject"`
	Secondary         *semanticir.ArtifactRef          `json:"secondary,omitempty"`
	Inputs            *semanticir.ArtifactRef          `json:"inputs,omitempty"`
	WorkspaceTree     string                           `json:"workspace_tree"`
	Argv              []string                         `json:"argv"`
	WorkingDirectory  string                           `json:"working_directory"`
	Environment       []semanticir.EnvironmentVariable `json:"environment"`
	EnvironmentDigest string                           `json:"environment_digest"`
	StdoutDigest      string                           `json:"stdout_digest"`
	StderrDigest      string                           `json:"stderr_digest"`
	ReportDigest      string                           `json:"report_digest"`
	Total             int                              `json:"total"`
	Complete          bool                             `json:"complete"`
}

type dependencyDiagnosticEvidence struct {
	Mode     string                     `json:"mode"`
	Reason   string                     `json:"reason,omitempty"`
	Run      *commandDiagnosticEvidence `json:"run,omitempty"`
	Complete bool                       `json:"complete"`
}

type diagnosticReport struct {
	Schema          string `json:"schema"`
	Kind            string `json:"kind"`
	Status          string `json:"status"`
	SpecIRDigest    string `json:"spec_ir_digest"`
	SubjectDigest   string `json:"subject_digest"`
	SecondaryDigest string `json:"secondary_digest,omitempty"`
	InputsDigest    string `json:"inputs_digest,omitempty"`
	Total           int    `json:"total"`
	Disagreements   int    `json:"disagreements"`
	Complete        bool   `json:"complete"`
}

func runDiagnostics(ctx context.Context, root string, cfg config, manifest taskbundle.Manifest, task *semanticir.Task) (diagnosticsEvidence, []string) {
	if task == nil || !semanticir.ValidDigest(task.SpecIRDigest) {
		return diagnosticsEvidence{}, []string{"diagnostics require canonical compiled Spec IR"}
	}
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return diagnosticsEvidence{}, []string{"stale task before diagnostics: " + err.Error()}
	}
	pict, err := runPICTDiagnostic(ctx, cfg.Diagnostics.PICT, manifest, task)
	if err != nil {
		return diagnosticsEvidence{}, []string{err.Error()}
	}
	oracle, err := runCommandDiagnostic(ctx, root, "oracle", "PROVED", cfg.Diagnostics.Oracle, manifest, task.SpecIRDigest)
	if err != nil {
		return diagnosticsEvidence{}, []string{err.Error()}
	}
	diff, err := runCommandDiagnostic(ctx, root, "diff-test", "AGREED", cfg.Diagnostics.DiffTest, manifest, task.SpecIRDigest)
	if err != nil {
		return diagnosticsEvidence{}, []string{err.Error()}
	}
	dependency := dependencyDiagnosticEvidence{Mode: cfg.Diagnostics.Dependency.Mode, Complete: true}
	if cfg.Diagnostics.Dependency.Mode == "not-applicable" {
		if len(manifest.RequiredInputs.DependencyArtifactIDs) != 0 {
			return diagnosticsEvidence{}, []string{"dependency diagnostic claims not-applicable but frozen dependency inputs are present"}
		}
		dependency.Reason = cfg.Diagnostics.Dependency.Reason
	} else {
		if !sameStringSet(cfg.Diagnostics.Dependency.DependencyInputs, manifest.RequiredInputs.DependencyArtifactIDs) {
			return diagnosticsEvidence{}, []string{"dependency diagnostic input IDs differ from frozen dependency roles"}
		}
		run, err := runCommandDiagnostic(ctx, root, "dep-harvest", "HARVESTED", cfg.Diagnostics.Dependency.Run, manifest, task.SpecIRDigest)
		if err != nil {
			return diagnosticsEvidence{}, []string{err.Error()}
		}
		dependency.Run = &run
	}
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		return diagnosticsEvidence{}, []string{"task changed during diagnostics: " + err.Error()}
	}
	return diagnosticsEvidence{SpecIRDigest: task.SpecIRDigest, PICT: pict, Oracle: oracle, DiffTest: diff, Dependency: dependency, Complete: true}, nil
}

func runPICTDiagnostic(ctx context.Context, cfg pictDiagnosticConfig, manifest taskbundle.Manifest, task *semanticir.Task) (pictDiagnosticEvidence, error) {
	tool, ok := manifest.Tool(cfg.ToolName)
	if !ok {
		return pictDiagnosticEvidence{}, fmt.Errorf("PICT tool %q is absent from the frozen environment", cfg.ToolName)
	}
	if err := taskbundle.VerifyTool(ctx, tool); err != nil {
		return pictDiagnosticEvidence{}, fmt.Errorf("verify PICT tool before run: %w", err)
	}
	model, allowed, err := pictModel(task.Domains)
	if err != nil {
		return pictDiagnosticEvidence{}, err
	}
	file, err := os.CreateTemp("", "ray-pict-model-*.txt")
	if err != nil {
		return pictDiagnosticEvidence{}, fmt.Errorf("create PICT model: %w", err)
	}
	modelPath := file.Name()
	defer os.Remove(modelPath)
	if _, err := file.Write(model); err != nil {
		file.Close()
		return pictDiagnosticEvidence{}, fmt.Errorf("write PICT model: %w", err)
	}
	if err := file.Close(); err != nil {
		return pictDiagnosticEvidence{}, fmt.Errorf("close PICT model: %w", err)
	}
	argv := []string{tool.Path, modelPath, "/d:|", "/o:" + strconv.Itoa(cfg.Strength)}
	stdout, stderr, err := runExactProcess(ctx, argv, "", nil, 60*time.Second)
	if err != nil {
		return pictDiagnosticEvidence{}, fmt.Errorf("PICT diagnostic: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	count, err := validatePICTOutput(stdout, allowed)
	if err != nil {
		return pictDiagnosticEvidence{}, err
	}
	if err := taskbundle.VerifyTool(ctx, tool); err != nil {
		return pictDiagnosticEvidence{}, fmt.Errorf("verify PICT tool after run: %w", err)
	}
	return pictDiagnosticEvidence{
		Tool: toolRef(tool), Strength: cfg.Strength, ModelDigest: semanticir.DigestBytes(model),
		Argv: argv, StdoutDigest: semanticir.DigestBytes(stdout), StderrDigest: semanticir.DigestBytes(stderr),
		Combinations: count, Complete: true,
	}, nil
}

func pictModel(domains []semanticir.Domain) ([]byte, map[string]map[string]bool, error) {
	if len(domains) == 0 {
		return nil, nil, errors.New("PICT diagnostic has no frozen Spec domains")
	}
	var model strings.Builder
	allowed := make(map[string]map[string]bool, len(domains))
	for _, domain := range domains {
		if domain.ID == "" || len(domain.Values) == 0 {
			return nil, nil, fmt.Errorf("PICT diagnostic domain %q is empty", domain.ID)
		}
		values := make([]string, 0, len(domain.Values))
		allowed[domain.ID] = map[string]bool{}
		for _, value := range domain.Values {
			if value.ID == "" || strings.ContainsAny(value.ID, "|\t\r\n") {
				return nil, nil, fmt.Errorf("PICT diagnostic domain %q has an unsafe value ID", domain.ID)
			}
			values = append(values, value.ID)
			allowed[domain.ID][value.ID] = true
		}
		fmt.Fprintf(&model, "%s: %s\n", domain.ID, strings.Join(values, " | "))
	}
	return []byte(model.String()), allowed, nil
}

func validatePICTOutput(output []byte, allowed map[string]map[string]bool) (int, error) {
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	var header []string
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if header == nil {
			header = fields
			if len(header) != len(allowed) {
				return 0, fmt.Errorf("PICT output header has %d domains, want %d", len(header), len(allowed))
			}
			seen := map[string]bool{}
			for _, name := range header {
				if allowed[name] == nil || seen[name] {
					return 0, fmt.Errorf("PICT output has unknown or repeated domain %q", name)
				}
				seen[name] = true
			}
			continue
		}
		if len(fields) != len(header) {
			return 0, errors.New("PICT output row cardinality differs from its header")
		}
		for index, value := range fields {
			if !allowed[header[index]][value] {
				return 0, fmt.Errorf("PICT output value %q is outside frozen domain %q", value, header[index])
			}
		}
		count++
	}
	if header == nil || count == 0 {
		return 0, errors.New("PICT produced no valid combinations")
	}
	return count, nil
}

func runCommandDiagnostic(ctx context.Context, root, kind, wantStatus string, cfg commandDiagnosticConfig, manifest taskbundle.Manifest, specIRDigest string) (commandDiagnosticEvidence, error) {
	tool, ok := manifest.Tool(cfg.ToolName)
	if !ok {
		return commandDiagnosticEvidence{}, fmt.Errorf("%s tool %q is absent from frozen environment", kind, cfg.ToolName)
	}
	if err := taskbundle.VerifyTool(ctx, tool); err != nil {
		return commandDiagnosticEvidence{}, fmt.Errorf("verify %s tool before run: %w", kind, err)
	}
	subject, err := diagnosticArtifact(manifest, cfg.ArtifactID)
	if err != nil {
		return commandDiagnosticEvidence{}, err
	}
	workspace, err := solutionManifestWorkspace(manifest)
	if err != nil {
		return commandDiagnosticEvidence{}, err
	}
	if err := requireWorkspaceBinding(workspace, cfg.WorkspacePath, subject); err != nil {
		return commandDiagnosticEvidence{}, fmt.Errorf("%s subject: %w", kind, err)
	}
	var secondary, inputs *semanticir.ArtifactRef
	if cfg.SecondaryArtifactID != "" {
		value, err := diagnosticArtifact(manifest, cfg.SecondaryArtifactID)
		if err != nil {
			return commandDiagnosticEvidence{}, err
		}
		if err := requireWorkspaceBinding(workspace, cfg.SecondaryPath, value); err != nil {
			return commandDiagnosticEvidence{}, fmt.Errorf("%s secondary: %w", kind, err)
		}
		secondary = &value
	}
	if cfg.InputsArtifactID != "" {
		value, err := diagnosticArtifact(manifest, cfg.InputsArtifactID)
		if err != nil {
			return commandDiagnosticEvidence{}, err
		}
		if err := requireWorkspaceBinding(workspace, cfg.InputsPath, value); err != nil {
			return commandDiagnosticEvidence{}, fmt.Errorf("%s inputs: %w", kind, err)
		}
		inputs = &value
	}
	for _, required := range []string{cfg.WorkspacePath, cfg.SecondaryPath, cfg.InputsPath} {
		if required != "" && !containsString(cfg.Argv, required) {
			return commandDiagnosticEvidence{}, fmt.Errorf("%s argv does not select frozen workspace path %q", kind, required)
		}
	}
	originalRoot := filepath.Join(root, filepath.FromSlash(workspace.Path))
	tempParent, copiedRoot, err := copyDiagnosticWorkspace(originalRoot)
	if err != nil {
		return commandDiagnosticEvidence{}, fmt.Errorf("copy %s workspace: %w", kind, err)
	}
	defer os.RemoveAll(tempParent)
	workDir := filepath.Join(copiedRoot, filepath.FromSlash(cfg.WorkingDirectory))
	if !withinRoot(copiedRoot, workDir) {
		return commandDiagnosticEvidence{}, fmt.Errorf("%s working directory escapes isolated workspace", kind)
	}
	environment := cloneMap(cfg.Environment)
	environment["RAY_SPEC_IR_SHA256"] = specIRDigest
	environment["RAY_SUBJECT_SHA256"] = subject.Digest
	if secondary != nil {
		environment["RAY_SECONDARY_SHA256"] = secondary.Digest
	}
	if inputs != nil {
		environment["RAY_INPUTS_SHA256"] = inputs.Digest
	}
	env := semanticEnvironment(environment)
	envDigest, err := semanticir.Digest(env)
	if err != nil {
		return commandDiagnosticEvidence{}, fmt.Errorf("digest %s environment: %w", kind, err)
	}
	argv := append([]string{tool.Path}, cfg.Argv...)
	stdout, stderr, err := runExactProcess(ctx, argv, workDir, env, time.Duration(cfg.TimeoutMillis)*time.Millisecond)
	if err != nil {
		return commandDiagnosticEvidence{}, fmt.Errorf("%s command: %w: %s", kind, err, strings.TrimSpace(string(stderr)))
	}
	report, reportDigest, err := decodeDiagnosticReport(stdout)
	if err != nil {
		return commandDiagnosticEvidence{}, fmt.Errorf("%s report: %w", kind, err)
	}
	wantSecondary, wantInputs := "", ""
	if secondary != nil {
		wantSecondary = secondary.Digest
	}
	if inputs != nil {
		wantInputs = inputs.Digest
	}
	if report.Kind != kind || report.Status != wantStatus || report.SpecIRDigest != specIRDigest || report.SubjectDigest != subject.Digest ||
		report.SecondaryDigest != wantSecondary || report.InputsDigest != wantInputs || report.Total <= 0 || report.Disagreements != 0 || !report.Complete {
		return commandDiagnosticEvidence{}, fmt.Errorf("%s report is incomplete, unknown, refuted, or detached from frozen inputs", kind)
	}
	if err := taskbundle.VerifyTool(ctx, tool); err != nil {
		return commandDiagnosticEvidence{}, fmt.Errorf("verify %s tool after run: %w", kind, err)
	}
	return commandDiagnosticEvidence{
		Kind: kind, Tool: toolRef(tool), Subject: subject, Secondary: secondary, Inputs: inputs,
		WorkspaceTree: workspace.TreeSHA256, Argv: argv, WorkingDirectory: cfg.WorkingDirectory,
		Environment: env, EnvironmentDigest: envDigest, StdoutDigest: semanticir.DigestBytes(stdout),
		StderrDigest: semanticir.DigestBytes(stderr), ReportDigest: reportDigest, Total: report.Total, Complete: true,
	}, nil
}

func decodeDiagnosticReport(source []byte) (diagnosticReport, string, error) {
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	var report diagnosticReport
	if err := decoder.Decode(&report); err != nil {
		return diagnosticReport{}, "", err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return diagnosticReport{}, "", errors.New("trailing diagnostic JSON")
	}
	canonical, err := semanticir.CanonicalJSON(report)
	if err != nil {
		return diagnosticReport{}, "", err
	}
	if !bytes.Equal(source, canonical) {
		return diagnosticReport{}, "", errors.New("diagnostic report is not canonical JSON")
	}
	if report.Schema != diagnosticReportSchema {
		return diagnosticReport{}, "", fmt.Errorf("schema %q, want %q", report.Schema, diagnosticReportSchema)
	}
	return report, semanticir.DigestBytes(canonical), nil
}

func runExactProcess(parent context.Context, argv []string, dir string, environment []semanticir.EnvironmentVariable, timeout time.Duration) ([]byte, []byte, error) {
	if len(argv) == 0 || !filepath.IsAbs(argv[0]) || timeout <= 0 {
		return nil, nil, errors.New("exact diagnostic command has invalid argv or timeout")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = make([]string, 0, len(environment))
	for _, variable := range environment {
		cmd.Env = append(cmd.Env, variable.Name+"="+variable.Value)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = maxDiagnosticOutput, maxDiagnosticOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if stdout.exceeded || stderr.exceeded {
			return stdout.Bytes(), stderr.Bytes(), errors.New("diagnostic output exceeded limit")
		}
		return stdout.Bytes(), stderr.Bytes(), err
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		return stdout.Bytes(), stderr.Bytes(), ctx.Err()
	}
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (buffer *limitedBuffer) Write(p []byte) (int, error) {
	if buffer.Len()+len(p) > buffer.limit {
		remaining := buffer.limit - buffer.Len()
		if remaining > 0 {
			_, _ = buffer.Buffer.Write(p[:remaining])
		}
		buffer.exceeded = true
		return len(p), nil
	}
	return buffer.Buffer.Write(p)
}

func copyDiagnosticWorkspace(source string) (string, string, error) {
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", "", err
	}
	parent, err := os.MkdirTemp(filepath.Dir(source), ".ray-diagnostic-*")
	if err != nil {
		return "", "", err
	}
	destination := filepath.Join(parent, "workspace")
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode().IsRegular():
			return copyDiagnosticFile(path, target, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if filepath.IsAbs(link) || !withinRoot(source, filepath.Clean(filepath.Join(filepath.Dir(path), link))) {
				return fmt.Errorf("workspace symlink %q escapes the frozen root", relative)
			}
			return os.Symlink(link, target)
		default:
			return fmt.Errorf("workspace entry %q has unsupported mode %s", relative, info.Mode())
		}
	}); err != nil {
		_ = os.RemoveAll(parent)
		return "", "", err
	}
	return parent, destination, nil
}

func copyDiagnosticFile(source, destination string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func diagnosticArtifact(manifest taskbundle.Manifest, id string) (semanticir.ArtifactRef, error) {
	for _, artifact := range manifest.Artifacts {
		if artifact.ID == id {
			return semanticir.ArtifactRef{ID: artifact.ID, Kind: semanticir.ArtifactKind(artifact.Kind), Path: artifact.Path, Digest: artifact.SHA256}, nil
		}
	}
	return semanticir.ArtifactRef{}, fmt.Errorf("diagnostic artifact %q is absent from frozen manifest", id)
}

func solutionManifestWorkspace(manifest taskbundle.Manifest) (taskbundle.Workspace, error) {
	for _, workspace := range manifest.Workspaces {
		if workspace.State == taskbundle.SolutionNewTests {
			return workspace, nil
		}
	}
	return taskbundle.Workspace{}, errors.New("frozen solution+new-tests workspace is absent")
}

func requireWorkspaceBinding(workspace taskbundle.Workspace, path string, artifact semanticir.ArtifactRef) error {
	clean, err := cleanTaskRelativePath(path)
	if err != nil {
		return err
	}
	for _, entry := range workspace.Entries {
		if filepath.ToSlash(entry.Path) == clean {
			if entry.Kind != "file" || entry.SHA256 != artifact.Digest {
				return fmt.Errorf("workspace entry %q differs from frozen artifact %q", clean, artifact.ID)
			}
			return nil
		}
	}
	return fmt.Errorf("workspace entry %q is absent", clean)
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func sameStringSet(left, right []string) bool {
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
