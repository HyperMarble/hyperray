// Package taskbundle freezes every input to a Hyperray verification run into a
// deterministic, SHA-256-bound manifest.
//
// A freeze is deliberately active: it executes the declared verifier command
// in each of the three required workspace states. Callers cannot assert that a
// command passed. The package captures the result and evaluates the task's
// declared pass signal itself.
package taskbundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"time"
)

const SchemaVersion = "hyperray.taskbundle.freeze/v1"

const maxCapturedCommandOutput = 16 << 20

// WorkspaceState names the three executions needed to demonstrate that the
// baseline is healthy, the new tests expose the task, and the reference
// solution satisfies those tests.
type WorkspaceState string

const (
	BaseOldTests     WorkspaceState = "base+old-tests"
	BaseNewTests     WorkspaceState = "base+new-tests"
	SolutionNewTests WorkspaceState = "base+solution+new-tests"
)

type SignalSource string

const (
	SignalExitCode SignalSource = "exit-code"
	SignalStdout   SignalSource = "stdout"
	SignalStderr   SignalSource = "stderr"
	SignalFile     SignalSource = "file"
)

type SignalMatch string

const (
	MatchExact    SignalMatch = "exact"
	MatchContains SignalMatch = "contains"
)

// ArtifactSpec declares a required regular file. Paths are relative to the
// task root; absolute paths and paths which escape the root are rejected.
type ArtifactSpec struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// StandardArtifactSpecs is the patch-shaped task bundle described by Hyperray's
// v0.1 design. A caller may extend or replace it, but every declaration is
// mandatory once supplied to Freeze.
func StandardArtifactSpecs() []ArtifactSpec {
	return []ArtifactSpec{
		{ID: "instruction", Kind: "instruction", Path: "instruction.md"},
		{ID: "spec", Kind: "spec", Path: "spec.md"},
		{ID: "task-config", Kind: "configuration", Path: "task.toml"},
		{ID: "environment", Kind: "environment", Path: "environment/Dockerfile"},
		{ID: "tests-patch", Kind: "tests", Path: "tests/test.patch"},
		{ID: "tests-command", Kind: "tests", Path: "tests/test.sh"},
		{ID: "solution-patch", Kind: "solution", Path: "solution/solution.patch"},
	}
}

type ToolVersion struct {
	Name                string   `json:"name"`
	Version             string   `json:"version"` // exact reported version; retained for compatibility
	ExpectedVersion     string   `json:"expected_version"`
	ReportedVersion     string   `json:"reported_version"`
	Path                string   `json:"path"`
	SHA256              string   `json:"sha256"`
	VersionArgs         []string `json:"version_args"`
	VersionOutputSHA256 string   `json:"version_output_sha256"`
}

// Environment is the caller's reproducible environment identity. Configuration
// is intentionally explicit rather than a snapshot of the ambient process
// environment, which often contains secrets and nondeterministic values.
type Environment struct {
	Identity      string            `json:"identity"`
	Configuration map[string]string `json:"configuration"`
	Tools         []ToolVersion     `json:"tools"`
}

// PassSignal says where the real verdict is found. Harbor-style verifiers can
// exit zero even on failure, so exit status is only one explicitly selectable
// source, never an implicit definition of success. File paths are relative to
// the workspace root.
type PassSignal struct {
	Source   SignalSource `json:"source"`
	Match    SignalMatch  `json:"match"`
	Expected string       `json:"expected"`
	Path     string       `json:"path,omitempty"`
}

// Command records the exact command and environment used in a workspace.
// Text is evaluated by Shell with "-c", matching Hyperray's command-string
// integration contract. WorkingDirectory is relative to the workspace root.
type Command struct {
	Text             string            `json:"text"`
	Shell            string            `json:"shell"`
	ShellToolName    string            `json:"shell_tool_name"`
	WorkingDirectory string            `json:"working_directory"`
	Environment      map[string]string `json:"environment"`
	TimeoutMillis    int64             `json:"timeout_millis"`
	PassSignal       PassSignal        `json:"pass_signal"`
}

// WorkspaceInput refers to a prepared workspace. Root may be absolute or
// relative on input, but must resolve beneath the task root; the manifest stores
// only its normalized relative path.
type WorkspaceInput struct {
	State      WorkspaceState      `json:"state"`
	Root       string              `json:"root"`
	Derivation WorkspaceDerivation `json:"derivation"`
	Command    Command             `json:"command"`
}

// WorkspaceChange identifies one exact declared artifact whose bytes replace
// or add a file in a derived workspace. No undeclared file delta is allowed.
type WorkspaceChange struct {
	ArtifactID string `json:"artifact_id"`
	Path       string `json:"path"`
}

// WorkspaceDerivation makes the workspace triple cryptographically related:
// base+new-tests is base+old-tests plus declared test changes, and
// base+solution+new-tests is base+new-tests plus declared solution/code
// changes. The base workspace has an empty derivation.
type WorkspaceDerivation struct {
	Parent           WorkspaceState    `json:"parent,omitempty"`
	Changes          []WorkspaceChange `json:"changes"`
	PatchArtifactIDs []string          `json:"patch_artifact_ids,omitempty"`
}

// RepositoryInput identifies the immutable VCS commit from which patch-shaped
// workspace states must be reconstructed. Freeze never trusts a caller-supplied
// tree digest: it resolves and archives BaseCommit with the named frozen tool.
type RepositoryInput struct {
	Root          string            `json:"root"`
	BaseCommit    string            `json:"base_commit"`
	ToolName      string            `json:"tool_name"`
	Environment   map[string]string `json:"environment"`
	TimeoutMillis int64             `json:"timeout_millis"`
}

// RequiredInputs assigns every proof-critical task artifact its immutable
// role. Artifact kinds alone cannot distinguish public from hidden verifier
// inputs, so the role binding is part of the self-authenticating manifest.
type RequiredInputs struct {
	InstructionArtifactID  string      `json:"instruction_artifact_id"`
	SpecArtifactID         string      `json:"spec_artifact_id"`
	SolutionArtifactIDs    []string    `json:"solution_artifact_ids"`
	PublicTestArtifactIDs  []string    `json:"public_test_artifact_ids"`
	HiddenTestArtifactIDs  []string    `json:"hidden_test_artifact_ids"`
	EnvironmentArtifactIDs []string    `json:"environment_artifact_ids"`
	DependencyArtifactIDs  []string    `json:"dependency_artifact_ids"`
	RoleRanges             []RoleRange `json:"role_ranges,omitempty"`
}

// RoleRange binds one half-open byte slice of a mixed patch artifact to one
// proof role. It permits a single PR diff to contain test and solution hunks
// without duplicating or relabeling the frozen file.
type RoleRange struct {
	Role       string `json:"role"`
	ArtifactID string `json:"artifact_id"`
	StartByte  int64  `json:"start_byte"`
	EndByte    int64  `json:"end_byte"`
	SHA256     string `json:"sha256,omitempty"`
}

// Repository records the exact VCS queries and exported base tree used for
// workspace reconstruction. Argv entries are the actual absolute invocations;
// stdout/stderr and archive bytes are digest-bound.
type Repository struct {
	Path                string            `json:"path"`
	BaseCommit          string            `json:"base_commit"`
	BaseTreeSHA256      string            `json:"base_tree_sha256"`
	ArchiveSHA256       string            `json:"archive_sha256"`
	Tool                ToolVersion       `json:"tool"`
	ResolveArgv         []string          `json:"resolve_argv"`
	ArchiveArgv         []string          `json:"archive_argv"`
	Environment         map[string]string `json:"environment"`
	EnvironmentSHA256   string            `json:"environment_sha256"`
	TimeoutMillis       int64             `json:"timeout_millis"`
	ResolveStdoutSHA256 string            `json:"resolve_stdout_sha256"`
	ResolveStderrSHA256 string            `json:"resolve_stderr_sha256"`
	ArchiveStderrSHA256 string            `json:"archive_stderr_sha256"`
}

// PatchApplication is one ordered, freshly executed transformation from the
// archived base commit. The retained Artifact fields prevent patch relabeling;
// ResultTreeSHA256 makes every intermediate state auditable.
type PatchApplication struct {
	ArtifactID        string            `json:"artifact_id"`
	ArtifactKind      string            `json:"artifact_kind"`
	ArtifactPath      string            `json:"artifact_path"`
	ArtifactSHA256    string            `json:"artifact_sha256"`
	InputSHA256       string            `json:"input_sha256"`
	RoleRanges        []RoleRange       `json:"role_ranges,omitempty"`
	Tool              ToolVersion       `json:"tool"`
	Argv              []string          `json:"argv"`
	Environment       map[string]string `json:"environment"`
	EnvironmentSHA256 string            `json:"environment_sha256"`
	TimeoutMillis     int64             `json:"timeout_millis"`
	StdoutSHA256      string            `json:"stdout_sha256"`
	StderrSHA256      string            `json:"stderr_sha256"`
	ResultTreeSHA256  string            `json:"result_tree_sha256"`
}

// PatchReplay proves a workspace was reconstructed from the exact archived
// base commit plus its complete ordered patch prefix.
type PatchReplay struct {
	BaseCommit       string             `json:"base_commit"`
	BaseTreeSHA256   string             `json:"base_tree_sha256"`
	Applications     []PatchApplication `json:"applications"`
	ResultTreeSHA256 string             `json:"result_tree_sha256"`
}

type Request struct {
	Artifacts      []ArtifactSpec   `json:"artifacts"`
	RequiredInputs RequiredInputs   `json:"required_inputs"`
	Environment    Environment      `json:"environment"`
	Repository     *RepositoryInput `json:"repository"`
	Workspaces     []WorkspaceInput `json:"workspaces"`
}

type Artifact struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type WorkspaceEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"` // file or symlink
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type CommandResult struct {
	ExitCode          int    `json:"exit_code"`
	Passed            bool   `json:"passed"`
	StdoutSHA256      string `json:"stdout_sha256"`
	StderrSHA256      string `json:"stderr_sha256"`
	SignalValueSHA256 string `json:"signal_value_sha256"`
}

type Workspace struct {
	State       WorkspaceState      `json:"state"`
	Path        string              `json:"path"`
	Derivation  WorkspaceDerivation `json:"derivation"`
	TreeSHA256  string              `json:"tree_sha256"`
	Entries     []WorkspaceEntry    `json:"entries"`
	PatchReplay *PatchReplay        `json:"patch_replay,omitempty"`
	Command     Command             `json:"command"`
	Result      CommandResult       `json:"result"`
}

// Manifest is self-authenticating: SHA256 is calculated over its canonical JSON
// with the SHA256 field empty. It is an integrity binding, not a signature.
type Manifest struct {
	Schema         string         `json:"schema"`
	Artifacts      []Artifact     `json:"artifacts"`
	RequiredInputs RequiredInputs `json:"required_inputs"`
	Environment    Environment    `json:"environment"`
	Repository     *Repository    `json:"repository"`
	Workspaces     []Workspace    `json:"workspaces"`
	SHA256         string         `json:"sha256"`
}

// Tool resolves a proof-critical tool identity by its stable logical name.
// The returned VersionArgs slice is copied so callers cannot mutate a manifest
// through the lookup result.
func (m Manifest) Tool(name string) (ToolVersion, bool) {
	for _, tool := range m.Environment.Tools {
		if tool.Name == name {
			tool.VersionArgs = append([]string{}, tool.VersionArgs...)
			return tool, true
		}
	}
	return ToolVersion{}, false
}

// LoadRequest strictly decodes a JSON freeze request. Unknown fields and
// trailing JSON are rejected so misspelled security-relevant settings cannot
// be silently ignored.
func LoadRequest(path string) (Request, error) {
	f, err := os.Open(path)
	if err != nil {
		return Request{}, fmt.Errorf("open freeze request: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var req Request
	if err := dec.Decode(&req); err != nil {
		return Request{}, fmt.Errorf("decode freeze request: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Request{}, errors.New("decode freeze request: trailing JSON value")
		}
		return Request{}, fmt.Errorf("decode freeze request trailing data: %w", err)
	}
	return req, nil
}

// Freeze executes and seals a request using context.Background.
func Freeze(root string, req Request) (Manifest, error) {
	return FreezeContext(context.Background(), root, req)
}

// FreezeContext hashes all artifacts and workspace trees before executing each
// workspace command in an isolated copy. It returns no manifest unless the
// complete workspace triple has the required pass/fail/pass observations.
func FreezeContext(ctx context.Context, root string, req Request) (Manifest, error) {
	if ctx == nil {
		return Manifest{}, errors.New("freeze: context is nil")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve task root: %w", err)
	}
	info, err := os.Stat(rootAbs)
	if err != nil {
		return Manifest{}, fmt.Errorf("task root: %w", err)
	}
	if !info.IsDir() {
		return Manifest{}, fmt.Errorf("task root %q is not a directory", root)
	}

	env, err := freezeEnvironment(ctx, req.Environment)
	if err != nil {
		return Manifest{}, err
	}
	artifacts, err := freezeArtifacts(rootAbs, req.Artifacts)
	if err != nil {
		return Manifest{}, err
	}
	requiredInputs, err := normalizeRequiredInputs(rootAbs, req.RequiredInputs, artifacts)
	if err != nil {
		return Manifest{}, err
	}
	if req.Repository == nil {
		return Manifest{}, errors.New("freeze: repository and exact base commit are required")
	}
	repository, archive, err := freezeRepository(ctx, rootAbs, req.Repository, env.Tools)
	if err != nil {
		return Manifest{}, err
	}
	workspaces, err := freezeWorkspaces(ctx, rootAbs, req.Workspaces, artifacts, requiredInputs, env.Tools, repository, archive)
	if err != nil {
		return Manifest{}, err
	}
	m := Manifest{
		Schema:         SchemaVersion,
		Artifacts:      artifacts,
		RequiredInputs: requiredInputs,
		Environment:    env,
		Repository:     repository,
		Workspaces:     workspaces,
	}
	digest, err := manifestBodyDigest(m)
	if err != nil {
		return Manifest{}, err
	}
	m.SHA256 = digest
	if err := Validate(m); err != nil {
		return Manifest{}, fmt.Errorf("validate frozen manifest: %w", err)
	}
	if err := Verify(rootAbs, m); err != nil {
		return Manifest{}, fmt.Errorf("verify in-run freeze: %w", err)
	}
	return m, nil
}

func freezeArtifacts(root string, specs []ArtifactSpec) ([]Artifact, error) {
	if len(specs) == 0 {
		return nil, errors.New("freeze: no artifacts declared")
	}
	seenID := make(map[string]bool, len(specs))
	seenPath := make(map[string]bool, len(specs))
	out := make([]Artifact, 0, len(specs))
	for _, spec := range specs {
		spec.ID = strings.TrimSpace(spec.ID)
		spec.Kind = strings.TrimSpace(spec.Kind)
		if spec.ID == "" || spec.Kind == "" {
			return nil, errors.New("freeze: artifact id and kind are required")
		}
		full, rel, err := resolveWithin(root, spec.Path)
		if err != nil {
			return nil, fmt.Errorf("artifact %q: %w", spec.ID, err)
		}
		if err := ensureResolvedWithin(root, full); err != nil {
			return nil, fmt.Errorf("artifact %q: %w", spec.ID, err)
		}
		if seenID[spec.ID] {
			return nil, fmt.Errorf("freeze: duplicate artifact id %q", spec.ID)
		}
		if seenPath[rel] {
			return nil, fmt.Errorf("freeze: duplicate artifact path %q", rel)
		}
		seenID[spec.ID] = true
		seenPath[rel] = true
		info, err := os.Lstat(full)
		if err != nil {
			return nil, fmt.Errorf("artifact %q (%s): %w", spec.ID, rel, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("artifact %q (%s) is not a regular file", spec.ID, rel)
		}
		digest, size, err := hashFile(full)
		if err != nil {
			return nil, fmt.Errorf("artifact %q (%s): %w", spec.ID, rel, err)
		}
		out = append(out, Artifact{
			ID: spec.ID, Kind: spec.Kind, Path: rel,
			Mode: uint32(info.Mode().Perm()), Size: size, SHA256: digest,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func normalizeRequiredInputs(root string, inputs RequiredInputs, artifacts []Artifact) (RequiredInputs, error) {
	inputs.InstructionArtifactID = strings.TrimSpace(inputs.InstructionArtifactID)
	inputs.SpecArtifactID = strings.TrimSpace(inputs.SpecArtifactID)
	if inputs.InstructionArtifactID == "" || inputs.SpecArtifactID == "" {
		return RequiredInputs{}, errors.New("freeze: required input instruction_artifact_id and spec_artifact_id are required")
	}
	lists := []struct {
		name     string
		role     string
		kind     string
		required bool
		ids      *[]string
	}{
		{"solution_artifact_ids", "solution", "solution", true, &inputs.SolutionArtifactIDs},
		{"public_test_artifact_ids", "public-test", "tests", true, &inputs.PublicTestArtifactIDs},
		{"hidden_test_artifact_ids", "hidden-test", "tests", true, &inputs.HiddenTestArtifactIDs},
		{"environment_artifact_ids", "environment", "environment", true, &inputs.EnvironmentArtifactIDs},
		{"dependency_artifact_ids", "dependency", "dependency", false, &inputs.DependencyArtifactIDs},
	}
	byID := make(map[string]Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	memberships := make(map[string][]string, len(artifacts))
	bind := func(role, id string) error {
		id = strings.TrimSpace(id)
		if _, exists := byID[id]; !exists {
			return fmt.Errorf("freeze: required input %s references undeclared artifact %q", role, id)
		}
		for _, previous := range memberships[id] {
			if previous == role {
				return fmt.Errorf("freeze: required input %s repeats artifact %q", role, id)
			}
		}
		memberships[id] = append(memberships[id], role)
		return nil
	}
	if err := bind("instruction", inputs.InstructionArtifactID); err != nil {
		return RequiredInputs{}, err
	}
	if err := bind("spec", inputs.SpecArtifactID); err != nil {
		return RequiredInputs{}, err
	}
	for _, list := range lists {
		if list.required && len(*list.ids) == 0 {
			return RequiredInputs{}, fmt.Errorf("freeze: required input %s must not be empty", list.name)
		}
		for index := range *list.ids {
			(*list.ids)[index] = strings.TrimSpace((*list.ids)[index])
			if err := bind(list.role, (*list.ids)[index]); err != nil {
				return RequiredInputs{}, err
			}
		}
		sort.Strings(*list.ids)
	}
	allowedRangeRoles := map[string]bool{"solution": true, "public-test": true, "hidden-test": true}
	rangesByArtifactRole := make(map[string]map[string]int)
	inputs.RoleRanges = append([]RoleRange(nil), inputs.RoleRanges...)
	for index := range inputs.RoleRanges {
		rangeBinding := &inputs.RoleRanges[index]
		rangeBinding.Role = strings.TrimSpace(rangeBinding.Role)
		rangeBinding.ArtifactID = strings.TrimSpace(rangeBinding.ArtifactID)
		artifact, exists := byID[rangeBinding.ArtifactID]
		if !allowedRangeRoles[rangeBinding.Role] || !exists {
			return RequiredInputs{}, fmt.Errorf("freeze: role range %d has an unknown role or artifact", index)
		}
		member := false
		for _, role := range memberships[rangeBinding.ArtifactID] {
			member = member || role == rangeBinding.Role
		}
		if !member || artifact.Kind != "patch" || rangeBinding.StartByte < 0 || rangeBinding.EndByte <= rangeBinding.StartByte || rangeBinding.EndByte > artifact.Size {
			return RequiredInputs{}, fmt.Errorf("freeze: role range %d is detached, empty, out of bounds, or not backed by kind patch", index)
		}
		if rangesByArtifactRole[rangeBinding.ArtifactID] == nil {
			rangesByArtifactRole[rangeBinding.ArtifactID] = map[string]int{}
		}
		rangesByArtifactRole[rangeBinding.ArtifactID][rangeBinding.Role]++
		if root != "" {
			full, _, err := resolveWithin(root, artifact.Path)
			if err != nil {
				return RequiredInputs{}, err
			}
			file, err := os.Open(full)
			if err != nil {
				return RequiredInputs{}, err
			}
			content := make([]byte, rangeBinding.EndByte-rangeBinding.StartByte)
			_, readErr := file.ReadAt(content, rangeBinding.StartByte)
			closeErr := file.Close()
			if readErr != nil || closeErr != nil {
				return RequiredInputs{}, fmt.Errorf("freeze: read role range %d: %v %v", index, readErr, closeErr)
			}
			observed := digestBytes(content)
			if rangeBinding.SHA256 != "" && rangeBinding.SHA256 != observed {
				return RequiredInputs{}, fmt.Errorf("freeze: role range %d caller digest mismatch", index)
			}
			rangeBinding.SHA256 = observed
		} else if !validDigest(rangeBinding.SHA256) {
			return RequiredInputs{}, fmt.Errorf("manifest required input role range %d has invalid digest", index)
		}
	}
	sort.Slice(inputs.RoleRanges, func(i, j int) bool {
		left, right := inputs.RoleRanges[i], inputs.RoleRanges[j]
		if left.ArtifactID != right.ArtifactID {
			return left.ArtifactID < right.ArtifactID
		}
		if left.StartByte != right.StartByte {
			return left.StartByte < right.StartByte
		}
		if left.EndByte != right.EndByte {
			return left.EndByte < right.EndByte
		}
		return left.Role < right.Role
	})
	for index := 1; index < len(inputs.RoleRanges); index++ {
		previous, current := inputs.RoleRanges[index-1], inputs.RoleRanges[index]
		if previous.ArtifactID == current.ArtifactID && current.StartByte < previous.EndByte {
			return RequiredInputs{}, fmt.Errorf("freeze: role ranges for artifact %q overlap", current.ArtifactID)
		}
	}
	expectedKinds := map[string]string{"instruction": "instruction", "spec": "spec", "solution": "solution", "public-test": "tests", "hidden-test": "tests", "environment": "environment", "dependency": "dependency"}
	for artifactID, roles := range memberships {
		artifact := byID[artifactID]
		if len(roles) > 1 || rangesByArtifactRole[artifactID] != nil {
			if artifact.Kind != "patch" {
				return RequiredInputs{}, fmt.Errorf("freeze: artifact %q is assigned to multiple roles without disjoint kind patch ranges", artifactID)
			}
			for _, role := range roles {
				if !allowedRangeRoles[role] || rangesByArtifactRole[artifactID][role] == 0 {
					return RequiredInputs{}, fmt.Errorf("freeze: mixed patch artifact %q role %s has no exact byte range", artifactID, role)
				}
			}
		} else if artifact.Kind != expectedKinds[roles[0]] {
			return RequiredInputs{}, fmt.Errorf("freeze: required input %s artifact %q has kind %q, want %q", roles[0], artifactID, artifact.Kind, expectedKinds[roles[0]])
		}
	}
	for _, artifact := range artifacts {
		switch artifact.Kind {
		case "instruction", "spec", "solution", "tests", "environment", "dependency", "patch":
			if _, exists := memberships[artifact.ID]; !exists {
				return RequiredInputs{}, fmt.Errorf("freeze: proof-critical artifact %q of kind %q has no required-input role", artifact.ID, artifact.Kind)
			}
		}
	}
	return inputs, nil
}

func requiredInputsEqual(left, right RequiredInputs) bool {
	if left.InstructionArtifactID != right.InstructionArtifactID || left.SpecArtifactID != right.SpecArtifactID {
		return false
	}
	pairs := [][2][]string{
		{left.SolutionArtifactIDs, right.SolutionArtifactIDs},
		{left.PublicTestArtifactIDs, right.PublicTestArtifactIDs},
		{left.HiddenTestArtifactIDs, right.HiddenTestArtifactIDs},
		{left.EnvironmentArtifactIDs, right.EnvironmentArtifactIDs},
		{left.DependencyArtifactIDs, right.DependencyArtifactIDs},
	}
	for _, pair := range pairs {
		if len(pair[0]) != len(pair[1]) {
			return false
		}
		for index := range pair[0] {
			if pair[0][index] != pair[1][index] {
				return false
			}
		}
	}
	if len(left.RoleRanges) != len(right.RoleRanges) {
		return false
	}
	for index := range left.RoleRanges {
		if left.RoleRanges[index] != right.RoleRanges[index] {
			return false
		}
	}
	return true
}

func freezeWorkspaces(ctx context.Context, root string, inputs []WorkspaceInput, artifacts []Artifact, requiredInputs RequiredInputs, tools []ToolVersion, repository *Repository, archive []byte) ([]Workspace, error) {
	if len(inputs) != 3 {
		return nil, fmt.Errorf("freeze: workspace triple requires exactly 3 states, got %d", len(inputs))
	}
	byState := make(map[WorkspaceState]WorkspaceInput, 3)
	var seenRoots []string
	for _, in := range inputs {
		if _, ok := expectedPass(in.State); !ok {
			return nil, fmt.Errorf("freeze: unknown workspace state %q", in.State)
		}
		if _, exists := byState[in.State]; exists {
			return nil, fmt.Errorf("freeze: duplicate workspace state %q", in.State)
		}
		_, rel, err := resolveWithin(root, in.Root)
		if err != nil {
			return nil, fmt.Errorf("workspace %q: %w", in.State, err)
		}
		for _, previous := range seenRoots {
			if pathsOverlap(previous, rel) {
				return nil, fmt.Errorf("freeze: workspace roots %q and %q overlap", previous, rel)
			}
		}
		seenRoots = append(seenRoots, rel)
		byState[in.State] = in
	}

	states := []WorkspaceState{BaseOldTests, BaseNewTests, SolutionNewTests}
	out := make([]Workspace, 0, len(states))
	for _, state := range states {
		in, ok := byState[state]
		if !ok {
			return nil, fmt.Errorf("freeze: missing workspace state %q", state)
		}
		full, rel, _ := resolveWithin(root, in.Root)
		if err := ensureResolvedWithin(root, full); err != nil {
			return nil, fmt.Errorf("workspace %q: %w", state, err)
		}
		info, err := os.Stat(full)
		if err != nil {
			return nil, fmt.Errorf("workspace %q (%s): %w", state, rel, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("workspace %q (%s) is not a directory", state, rel)
		}
		cmd, err := normalizeCommand(in.Command)
		if err != nil {
			return nil, fmt.Errorf("workspace %q: %w", state, err)
		}
		cmd, err = bindCommandShell(cmd, tools)
		if err != nil {
			return nil, fmt.Errorf("workspace %q: %w", state, err)
		}
		entries, treeDigest, err := snapshotTree(full)
		if err != nil {
			return nil, fmt.Errorf("workspace %q: %w", state, err)
		}
		derivation, err := normalizeWorkspaceDerivation(state, in.Derivation)
		if err != nil {
			return nil, fmt.Errorf("workspace %q: %w", state, err)
		}
		if err := validateWorkspaceDerivation(state, derivation, entries, out, artifacts, requiredInputs, repository); err != nil {
			return nil, fmt.Errorf("workspace %q: %w", state, err)
		}
		var patchReplay *PatchReplay
		if repository != nil {
			replayed, replayErr := replayWorkspacePatches(ctx, root, archive, *repository, state, derivation.PatchArtifactIDs, artifacts, requiredInputs)
			if replayErr != nil {
				return nil, fmt.Errorf("workspace %q patch replay: %w", state, replayErr)
			}
			if replayed.ResultTreeSHA256 != treeDigest {
				return nil, fmt.Errorf("workspace %q prepared tree differs from base+ordered-patch replay", state)
			}
			patchReplay = &replayed
		}
		result, err := executeIsolated(ctx, full, cmd, entries, treeDigest)
		if err != nil {
			return nil, fmt.Errorf("workspace %q: %w", state, err)
		}
		out = append(out, Workspace{
			State: state, Path: rel, Derivation: derivation, TreeSHA256: treeDigest, Entries: entries,
			PatchReplay: patchReplay, Command: cmd, Result: result,
		})
	}
	return out, nil
}

func normalizeWorkspaceDerivation(state WorkspaceState, derivation WorkspaceDerivation) (WorkspaceDerivation, error) {
	derivation.Changes = append([]WorkspaceChange(nil), derivation.Changes...)
	derivation.PatchArtifactIDs = append([]string(nil), derivation.PatchArtifactIDs...)
	seenPatches := make(map[string]bool, len(derivation.PatchArtifactIDs))
	for index := range derivation.PatchArtifactIDs {
		derivation.PatchArtifactIDs[index] = strings.TrimSpace(derivation.PatchArtifactIDs[index])
		if derivation.PatchArtifactIDs[index] == "" {
			return WorkspaceDerivation{}, errors.New("derivation patch artifact id is required")
		}
		if seenPatches[derivation.PatchArtifactIDs[index]] {
			return WorkspaceDerivation{}, errors.New("derivation patch artifact ids must be unique")
		}
		seenPatches[derivation.PatchArtifactIDs[index]] = true
	}
	if len(derivation.Changes) != 0 && len(derivation.PatchArtifactIDs) != 0 {
		return WorkspaceDerivation{}, errors.New("derivation cannot mix whole-file changes with ordered patch replay")
	}
	for index := range derivation.Changes {
		change := &derivation.Changes[index]
		change.ArtifactID = strings.TrimSpace(change.ArtifactID)
		if change.ArtifactID == "" {
			return WorkspaceDerivation{}, errors.New("derivation change artifact id is required")
		}
		clean, err := cleanRelative(change.Path)
		if err != nil {
			return WorkspaceDerivation{}, fmt.Errorf("derivation target %q: %w", change.Path, err)
		}
		change.Path = clean
	}
	sort.Slice(derivation.Changes, func(i, j int) bool {
		if derivation.Changes[i].Path != derivation.Changes[j].Path {
			return derivation.Changes[i].Path < derivation.Changes[j].Path
		}
		return derivation.Changes[i].ArtifactID < derivation.Changes[j].ArtifactID
	})
	switch state {
	case BaseOldTests:
		if derivation.Parent != "" || len(derivation.Changes) != 0 || len(derivation.PatchArtifactIDs) != 0 {
			return WorkspaceDerivation{}, errors.New("base+old-tests must be the derivation root with no changes")
		}
	case BaseNewTests:
		if derivation.Parent != BaseOldTests || len(derivation.Changes) == 0 && len(derivation.PatchArtifactIDs) == 0 {
			return WorkspaceDerivation{}, errors.New("base+new-tests must derive from base+old-tests using declared test changes")
		}
	case SolutionNewTests:
		if derivation.Parent != BaseNewTests || len(derivation.Changes) == 0 && len(derivation.PatchArtifactIDs) == 0 {
			return WorkspaceDerivation{}, errors.New("base+solution+new-tests must derive from base+new-tests using declared solution changes")
		}
	default:
		return WorkspaceDerivation{}, fmt.Errorf("unknown workspace state %q", state)
	}
	for index := 1; index < len(derivation.Changes); index++ {
		if derivation.Changes[index-1].Path == derivation.Changes[index].Path || derivation.Changes[index-1].ArtifactID == derivation.Changes[index].ArtifactID {
			return WorkspaceDerivation{}, errors.New("derivation changes require unique target paths and artifact ids")
		}
	}
	return derivation, nil
}

func validateWorkspaceDerivation(state WorkspaceState, derivation WorkspaceDerivation, entries []WorkspaceEntry, prior []Workspace, artifacts []Artifact, requiredInputs RequiredInputs, repository *Repository) error {
	if repository != nil {
		if len(derivation.Changes) != 0 {
			return errors.New("repository-backed workspace derivation cannot use caller-asserted whole-file changes")
		}
		artifactsByID := make(map[string]Artifact, len(artifacts))
		for _, artifact := range artifacts {
			artifactsByID[artifact.ID] = artifact
		}
		for index, artifactID := range derivation.PatchArtifactIDs {
			artifact, exists := artifactsByID[artifactID]
			if !exists {
				return fmt.Errorf("derivation references undeclared patch artifact %q", artifactID)
			}
			wantKind := "tests"
			if state == SolutionNewTests {
				var basePatchCount int
				for _, workspace := range prior {
					if workspace.State == BaseNewTests {
						basePatchCount = len(workspace.Derivation.PatchArtifactIDs)
						if index < basePatchCount && artifactID != workspace.Derivation.PatchArtifactIDs[index] {
							return errors.New("solution patch replay does not preserve the exact ordered test-patch prefix")
						}
					}
				}
				if index >= basePatchCount {
					wantKind = "solution"
				}
			}
			if artifact.Kind != wantKind && artifact.Kind != "patch" && !(wantKind == "solution" && artifact.Kind == "code") {
				return fmt.Errorf("patch artifact %q kind %q is invalid for %s replay position %d", artifact.ID, artifact.Kind, state, index)
			}
		}
		switch state {
		case BaseOldTests:
			if len(derivation.PatchArtifactIDs) != 0 {
				return errors.New("base+old-tests patch replay must contain no patches")
			}
		case BaseNewTests:
			if len(derivation.PatchArtifactIDs) == 0 {
				return errors.New("base+new-tests patch replay requires declared test patches")
			}
		case SolutionNewTests:
			baseCount := -1
			for _, workspace := range prior {
				if workspace.State == BaseNewTests {
					baseCount = len(workspace.Derivation.PatchArtifactIDs)
				}
			}
			sharedSolutionRange := false
			if baseCount >= 0 && len(derivation.PatchArtifactIDs) == baseCount {
				for _, artifactID := range derivation.PatchArtifactIDs {
					for _, roleRange := range requiredInputs.RoleRanges {
						sharedSolutionRange = sharedSolutionRange || roleRange.ArtifactID == artifactID && roleRange.Role == "solution"
					}
				}
			}
			if baseCount < 0 || len(derivation.PatchArtifactIDs) < baseCount || (len(derivation.PatchArtifactIDs) == baseCount && !sharedSolutionRange) {
				return errors.New("solution patch replay must append declared solution patches after all test patches")
			}
		}
		return nil
	}
	if len(derivation.PatchArtifactIDs) != 0 {
		return errors.New("ordered patch derivation requires a frozen repository identity")
	}
	if state == BaseOldTests {
		return nil
	}
	var parent *Workspace
	for index := range prior {
		if prior[index].State == derivation.Parent {
			parent = &prior[index]
			break
		}
	}
	if parent == nil {
		return fmt.Errorf("derivation parent %q is unavailable", derivation.Parent)
	}
	artifactsByID := make(map[string]Artifact, len(artifacts))
	for _, artifact := range artifacts {
		artifactsByID[artifact.ID] = artifact
	}
	expected := make(map[string]WorkspaceEntry, len(parent.Entries)+len(derivation.Changes))
	for _, entry := range parent.Entries {
		expected[entry.Path] = entry
	}
	for _, change := range derivation.Changes {
		artifact, ok := artifactsByID[change.ArtifactID]
		if !ok {
			return fmt.Errorf("derivation references undeclared artifact %q", change.ArtifactID)
		}
		allowed := false
		switch state {
		case BaseNewTests:
			allowed = artifact.Kind == "tests" || artifact.Kind == "command"
		case SolutionNewTests:
			allowed = artifact.Kind == "solution" || artifact.Kind == "code"
		}
		if !allowed {
			return fmt.Errorf("artifact %q kind %q is invalid for %s derivation", artifact.ID, artifact.Kind, state)
		}
		next := WorkspaceEntry{Path: change.Path, Kind: "file", Mode: artifact.Mode, Size: artifact.Size, SHA256: artifact.SHA256}
		if previous, exists := expected[change.Path]; exists && previous == next {
			return fmt.Errorf("derivation change %q is a no-op", change.Path)
		}
		expected[change.Path] = next
	}
	want := make([]WorkspaceEntry, 0, len(expected))
	for _, entry := range expected {
		want = append(want, entry)
	}
	sort.Slice(want, func(i, j int) bool { return want[i].Path < want[j].Path })
	if !workspaceEntriesEqual(want, entries) {
		return errors.New("workspace tree has undeclared or missing changes relative to its derivation parent")
	}
	return nil
}

func freezeEnvironment(ctx context.Context, env Environment) (Environment, error) {
	env.Identity = strings.TrimSpace(env.Identity)
	if env.Identity == "" {
		return Environment{}, errors.New("freeze: environment identity is required")
	}
	if env.Configuration == nil {
		env.Configuration = map[string]string{}
	} else {
		env.Configuration = cloneMap(env.Configuration)
	}
	for k := range env.Configuration {
		if strings.TrimSpace(k) == "" {
			return Environment{}, errors.New("freeze: environment configuration contains an empty key")
		}
	}
	if len(env.Tools) == 0 {
		return Environment{}, errors.New("freeze: at least one tool version is required")
	}
	tools := append([]ToolVersion(nil), env.Tools...)
	for i := range tools {
		tools[i].Name = strings.TrimSpace(tools[i].Name)
		tools[i].ExpectedVersion = strings.TrimSpace(tools[i].ExpectedVersion)
		if tools[i].ExpectedVersion == "" {
			tools[i].ExpectedVersion = strings.TrimSpace(tools[i].Version)
		}
		if tools[i].Name == "" || tools[i].ExpectedVersion == "" {
			return Environment{}, errors.New("freeze: tool name and version are required")
		}
		if tools[i].Path == "" {
			tools[i].Path = tools[i].Name
		}
		resolved, err := exec.LookPath(tools[i].Path)
		if err != nil {
			return Environment{}, fmt.Errorf("freeze: tool %q executable %q: %w", tools[i].Name, tools[i].Path, err)
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return Environment{}, fmt.Errorf("freeze: tool %q path: %w", tools[i].Name, err)
		}
		// Preserve the absolute invocation path. Tool shims such as rustc ->
		// rustup dispatch from argv[0]; replacing that path with the symlink
		// target changes the program's semantics. hashFile still follows the
		// link and binds the executable bytes reached by this exact path.
		resolved = filepath.Clean(resolved)
		digest, _, err := hashFile(resolved)
		if err != nil {
			return Environment{}, fmt.Errorf("freeze: tool %q executable: %w", tools[i].Name, err)
		}
		if tools[i].SHA256 != "" && tools[i].SHA256 != digest {
			return Environment{}, fmt.Errorf("freeze: tool %q executable digest mismatch", tools[i].Name)
		}
		tools[i].Path = resolved
		tools[i].SHA256 = digest
		if tools[i].VersionArgs == nil {
			tools[i].VersionArgs = []string{"--version"}
		} else {
			tools[i].VersionArgs = append([]string{}, tools[i].VersionArgs...)
		}
		versionOutput, err := executeVersionCommand(ctx, tools[i])
		if err != nil {
			return Environment{}, err
		}
		requestedVersion := tools[i].ExpectedVersion
		if !bytes.Contains(versionOutput, []byte(requestedVersion)) {
			return Environment{}, fmt.Errorf("freeze: tool %q reported version does not contain %q", tools[i].Name, requestedVersion)
		}
		exactVersion := strings.TrimSpace(string(versionOutput))
		if exactVersion == "" {
			return Environment{}, fmt.Errorf("freeze: tool %q reported an empty version", tools[i].Name)
		}
		// The request's Version is a portable expected substring; the frozen
		// manifest retains the exact normalized version output which frontends
		// compare byte-for-byte against their own version query.
		tools[i].Version = exactVersion
		tools[i].ReportedVersion = exactVersion
		outputDigest := digestBytes(versionOutput)
		if tools[i].VersionOutputSHA256 != "" && tools[i].VersionOutputSHA256 != outputDigest {
			return Environment{}, fmt.Errorf("freeze: tool %q version evidence digest mismatch", tools[i].Name)
		}
		tools[i].VersionOutputSHA256 = outputDigest
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	for i := 1; i < len(tools); i++ {
		if tools[i-1].Name == tools[i].Name {
			return Environment{}, fmt.Errorf("freeze: duplicate tool %q", tools[i].Name)
		}
	}
	env.Tools = tools
	return env, nil
}

func validateEnvironment(env Environment) (Environment, error) {
	env.Identity = strings.TrimSpace(env.Identity)
	if env.Identity == "" {
		return Environment{}, errors.New("freeze: environment identity is required")
	}
	if env.Configuration == nil {
		env.Configuration = map[string]string{}
	} else {
		env.Configuration = cloneMap(env.Configuration)
	}
	for key := range env.Configuration {
		if strings.TrimSpace(key) == "" {
			return Environment{}, errors.New("freeze: environment configuration contains an empty key")
		}
	}
	if len(env.Tools) == 0 {
		return Environment{}, errors.New("freeze: at least one tool version is required")
	}
	env.Tools = append([]ToolVersion{}, env.Tools...)
	for i := range env.Tools {
		tool := &env.Tools[i]
		tool.Name = strings.TrimSpace(tool.Name)
		tool.Version = strings.TrimSpace(tool.Version)
		tool.ExpectedVersion = strings.TrimSpace(tool.ExpectedVersion)
		tool.ReportedVersion = strings.TrimSpace(tool.ReportedVersion)
		if tool.Name == "" || tool.Version == "" || tool.ExpectedVersion == "" || tool.ReportedVersion == "" || tool.Version != tool.ReportedVersion ||
			!strings.Contains(tool.ReportedVersion, tool.ExpectedVersion) || !filepath.IsAbs(tool.Path) || filepath.Clean(tool.Path) != tool.Path {
			return Environment{}, fmt.Errorf("freeze: tool %q has incomplete identity", tool.Name)
		}
		if !validDigest(tool.SHA256) || !validDigest(tool.VersionOutputSHA256) || tool.VersionArgs == nil {
			return Environment{}, fmt.Errorf("freeze: tool %q has invalid executable or version evidence", tool.Name)
		}
		tool.VersionArgs = append([]string{}, tool.VersionArgs...)
		for _, argument := range tool.VersionArgs {
			if strings.ContainsRune(argument, '\x00') {
				return Environment{}, fmt.Errorf("freeze: tool %q has an invalid version argument", tool.Name)
			}
		}
	}
	sort.Slice(env.Tools, func(i, j int) bool { return env.Tools[i].Name < env.Tools[j].Name })
	for i := 1; i < len(env.Tools); i++ {
		if env.Tools[i-1].Name == env.Tools[i].Name {
			return Environment{}, fmt.Errorf("freeze: duplicate tool %q", env.Tools[i].Name)
		}
	}
	return env, nil
}

func normalizeCommand(cmd Command) (Command, error) {
	cmd.Text = strings.TrimSpace(cmd.Text)
	if cmd.Text == "" {
		return Command{}, errors.New("command text is required")
	}
	if cmd.Shell == "" {
		cmd.Shell = "/bin/sh"
	}
	if !filepath.IsAbs(cmd.Shell) {
		return Command{}, errors.New("command shell must be an absolute path")
	}
	cmd.Shell = filepath.Clean(cmd.Shell)
	cmd.ShellToolName = strings.TrimSpace(cmd.ShellToolName)
	if cmd.WorkingDirectory == "" {
		cmd.WorkingDirectory = "."
	}
	cleanWork, err := cleanRelative(cmd.WorkingDirectory)
	if err != nil {
		return Command{}, fmt.Errorf("working directory: %w", err)
	}
	cmd.WorkingDirectory = cleanWork
	if cmd.TimeoutMillis == 0 {
		cmd.TimeoutMillis = int64((10 * time.Minute) / time.Millisecond)
	}
	if cmd.TimeoutMillis < 1 {
		return Command{}, errors.New("command timeout_millis must be positive")
	}
	if cmd.Environment == nil {
		cmd.Environment = map[string]string{}
	} else {
		cmd.Environment = cloneMap(cmd.Environment)
	}
	for k := range cmd.Environment {
		if strings.TrimSpace(k) == "" || strings.ContainsRune(k, '=') {
			return Command{}, fmt.Errorf("invalid command environment key %q", k)
		}
	}
	// A shell started with no PATH may synthesize an implementation-defined
	// default. Persist PATH= when it was undeclared so execution and evidence
	// both describe the same non-ambient lookup behavior.
	if _, declared := cmd.Environment["PATH"]; !declared {
		cmd.Environment["PATH"] = ""
	}
	if err := validatePassSignal(cmd.PassSignal); err != nil {
		return Command{}, err
	}
	if cmd.PassSignal.Source == SignalFile {
		clean, err := cleanRelative(cmd.PassSignal.Path)
		if err != nil {
			return Command{}, fmt.Errorf("pass signal file: %w", err)
		}
		cmd.PassSignal.Path = clean
	}
	return cmd, nil
}

func bindCommandShell(command Command, tools []ToolVersion) (Command, error) {
	var matches []ToolVersion
	for _, tool := range tools {
		if command.ShellToolName != "" && tool.Name != command.ShellToolName {
			continue
		}
		if filepath.Clean(tool.Path) == command.Shell {
			matches = append(matches, tool)
		}
	}
	if len(matches) != 1 {
		if command.ShellToolName == "" {
			return Command{}, fmt.Errorf("command shell %q must match exactly one frozen tool identity", command.Shell)
		}
		return Command{}, fmt.Errorf("command shell %q does not match frozen tool %q", command.Shell, command.ShellToolName)
	}
	command.ShellToolName = matches[0].Name
	return command, nil
}

func validatePassSignal(signal PassSignal) error {
	switch signal.Source {
	case SignalExitCode, SignalStdout, SignalStderr, SignalFile:
	default:
		return fmt.Errorf("unsupported pass signal source %q", signal.Source)
	}
	switch signal.Match {
	case MatchExact, MatchContains:
	default:
		return fmt.Errorf("unsupported pass signal match %q", signal.Match)
	}
	if signal.Source == SignalExitCode {
		if signal.Match != MatchExact {
			return errors.New("exit-code pass signals require exact matching")
		}
		if _, err := strconv.Atoi(signal.Expected); err != nil {
			return fmt.Errorf("exit-code expected value must be an integer: %w", err)
		}
		if signal.Path != "" {
			return errors.New("exit-code pass signal must not set path")
		}
	} else if signal.Source == SignalFile {
		if signal.Path == "" {
			return errors.New("file pass signal requires path")
		}
	} else if signal.Path != "" {
		return fmt.Errorf("%s pass signal must not set path", signal.Source)
	}
	return nil
}

func executeIsolated(parent context.Context, sourceRoot string, command Command, expectedEntries []WorkspaceEntry, expectedTreeDigest string) (CommandResult, error) {
	tempParent, err := os.MkdirTemp("", "hyperray-freeze-workspace-*")
	if err != nil {
		return CommandResult{}, fmt.Errorf("create isolated workspace: %w", err)
	}
	defer os.RemoveAll(tempParent)
	runRoot := filepath.Join(tempParent, "workspace")
	if err := copyTree(sourceRoot, runRoot); err != nil {
		return CommandResult{}, fmt.Errorf("copy isolated workspace: %w", err)
	}
	copiedEntries, copiedDigest, err := snapshotTree(runRoot)
	if err != nil {
		return CommandResult{}, fmt.Errorf("snapshot isolated workspace: %w", err)
	}
	if copiedDigest != expectedTreeDigest || !workspaceEntriesEqual(copiedEntries, expectedEntries) {
		return CommandResult{}, errors.New("workspace changed while creating isolated execution copy")
	}
	workDir, _, err := resolveWithin(runRoot, command.WorkingDirectory)
	if err != nil {
		return CommandResult{}, fmt.Errorf("resolve command working directory: %w", err)
	}
	if err := ensureResolvedWithin(runRoot, workDir); err != nil {
		return CommandResult{}, fmt.Errorf("resolve command working directory: %w", err)
	}
	info, err := os.Stat(workDir)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return CommandResult{}, fmt.Errorf("command working directory: %w", err)
	}
	// A file verdict must be fresh evidence from this execution. Removing the
	// copied value prevents a pre-seeded PASS/FAIL/PASS triple from certifying a
	// no-op command which never produced a result at all.
	if command.PassSignal.Source == SignalFile {
		signalPath, _, err := resolveWithin(runRoot, command.PassSignal.Path)
		if err != nil {
			return CommandResult{}, fmt.Errorf("resolve pass signal file: %w", err)
		}
		if err := ensureResolvedWithin(runRoot, filepath.Dir(signalPath)); err != nil {
			return CommandResult{}, fmt.Errorf("resolve pass signal directory: %w", err)
		}
		if info, err := os.Lstat(signalPath); err == nil {
			if info.IsDir() {
				return CommandResult{}, fmt.Errorf("pass signal path %q is a directory", command.PassSignal.Path)
			}
			if err := os.Remove(signalPath); err != nil {
				return CommandResult{}, fmt.Errorf("remove stale pass signal %q: %w", command.PassSignal.Path, err)
			}
		} else if !os.IsNotExist(err) {
			return CommandResult{}, fmt.Errorf("inspect pass signal %q: %w", command.PassSignal.Path, err)
		}
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(command.TimeoutMillis)*time.Millisecond)
	defer cancel()
	cmd := exec.Command(command.Shell, "-c", command.Text)
	cmd.Dir = workDir
	// Certified commands receive exactly the declared environment. A nil Env
	// would inherit ambient credentials, PATH overrides, and locale settings.
	cmd.Env = exactEnvironment(command.Environment)
	stdout := newLimitedBuffer(maxCapturedCommandOutput)
	stderr := newLimitedBuffer(maxCapturedCommandOutput)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = 2 * time.Second
	runErr := runManagedProcess(ctx, cmd)
	if ctx.Err() != nil {
		return CommandResult{}, fmt.Errorf("command timed out after %dms: %w", command.TimeoutMillis, ctx.Err())
	}
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return CommandResult{}, fmt.Errorf("execute command: %w", runErr)
		}
		exitCode = exitErr.ExitCode()
	}
	if stdout.exceeded || stderr.exceeded {
		return CommandResult{}, fmt.Errorf("command output exceeded %d bytes", maxCapturedCommandOutput)
	}
	var signalValue []byte
	switch command.PassSignal.Source {
	case SignalExitCode:
		signalValue = []byte(strconv.Itoa(exitCode))
	case SignalStdout:
		signalValue = stdout.Bytes()
	case SignalStderr:
		signalValue = stderr.Bytes()
	case SignalFile:
		signalPath, _, err := resolveWithin(runRoot, command.PassSignal.Path)
		if err != nil {
			return CommandResult{}, fmt.Errorf("resolve pass signal file: %w", err)
		}
		if err := ensureResolvedWithin(runRoot, signalPath); err != nil {
			return CommandResult{}, fmt.Errorf("resolve pass signal file: %w", err)
		}
		signalValue, err = os.ReadFile(signalPath)
		if err != nil {
			return CommandResult{}, fmt.Errorf("read pass signal file %q: %w", command.PassSignal.Path, err)
		}
	}
	passed := string(signalValue) == command.PassSignal.Expected
	if command.PassSignal.Match == MatchContains {
		passed = bytes.Contains(signalValue, []byte(command.PassSignal.Expected))
	}
	return CommandResult{
		ExitCode: exitCode, Passed: passed,
		StdoutSHA256: digestBytes(stdout.Bytes()), StderrSHA256: digestBytes(stderr.Bytes()),
		SignalValueSHA256: digestBytes(signalValue),
	}, nil
}

func executeVersionCommand(parent context.Context, tool ToolVersion) ([]byte, error) {
	if parent == nil {
		return nil, fmt.Errorf("freeze: tool %q version context is nil", tool.Name)
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	cmd := exec.Command(tool.Path, tool.VersionArgs...)
	cmd.Env = []string{}
	output := newLimitedBuffer(maxCapturedCommandOutput)
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = 2 * time.Second
	err := runManagedProcess(ctx, cmd)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("freeze: tool %q version command timed out: %w", tool.Name, ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("freeze: tool %q version command failed: %w", tool.Name, err)
	}
	if output.exceeded {
		return nil, fmt.Errorf("freeze: tool %q version output exceeded %d bytes", tool.Name, maxCapturedCommandOutput)
	}
	return output.Bytes(), nil
}

func runManagedProcess(ctx context.Context, cmd *exec.Cmd) error {
	if ctx == nil {
		return errors.New("process context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Prefer a completed Wait over killing a process which happened to exit
		// concurrently with cancellation.
		select {
		case err := <-done:
			return err
		default:
			terminateProcess(cmd)
			<-done
			return ctx.Err()
		}
	}
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.exceeded = b.exceeded || originalLength > 0
		return originalLength, nil
	}
	if len(data) > remaining {
		b.exceeded = true
		data = data[:remaining]
	}
	_, err := b.Buffer.Write(data)
	return originalLength, err
}

// VerifyTool re-hashes a frozen executable and re-runs its exact version query.
// It is exported so translators can validate a Manifest.Tool result immediately
// before constructing their semanticir.ToolRef.
func VerifyTool(ctx context.Context, tool ToolVersion) error {
	validated, err := validateEnvironment(Environment{
		Identity: "tool-validation", Configuration: map[string]string{}, Tools: []ToolVersion{tool},
	})
	if err != nil {
		return err
	}
	tool = validated.Tools[0]
	digest, _, err := hashFile(tool.Path)
	if err != nil {
		return fmt.Errorf("verify tool %q (%s): %w", tool.Name, tool.Path, err)
	}
	if digest != tool.SHA256 {
		return fmt.Errorf("verify tool %q (%s): executable changed", tool.Name, tool.Path)
	}
	output, err := executeVersionCommand(ctx, tool)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(output)) != tool.ReportedVersion || digestBytes(output) != tool.VersionOutputSHA256 {
		return fmt.Errorf("verify tool %q (%s): version evidence changed", tool.Name, tool.Path)
	}
	return nil
}

// Verify rejects a stale or tampered manifest by re-hashing every declared
// artifact and workspace entry from the task root. Commands are not re-run;
// FreezeContext is the operation which obtains fresh execution evidence.
func Verify(root string, manifest Manifest) error {
	if err := Validate(manifest); err != nil {
		return err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve task root: %w", err)
	}
	for _, tool := range manifest.Environment.Tools {
		if err := VerifyTool(context.Background(), tool); err != nil {
			return err
		}
	}
	for _, artifact := range manifest.Artifacts {
		full, _, err := resolveWithin(rootAbs, artifact.Path)
		if err != nil {
			return fmt.Errorf("verify artifact %q: %w", artifact.ID, err)
		}
		if err := ensureResolvedWithin(rootAbs, full); err != nil {
			return fmt.Errorf("verify artifact %q: %w", artifact.ID, err)
		}
		info, err := os.Lstat(full)
		if err != nil {
			return fmt.Errorf("verify artifact %q (%s): %w", artifact.ID, artifact.Path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("verify artifact %q (%s): no longer a regular file", artifact.ID, artifact.Path)
		}
		digest, size, err := hashFile(full)
		if err != nil {
			return fmt.Errorf("verify artifact %q (%s): %w", artifact.ID, artifact.Path, err)
		}
		if digest != artifact.SHA256 || size != artifact.Size || uint32(info.Mode().Perm()) != artifact.Mode {
			return fmt.Errorf("verify artifact %q (%s): content or mode changed", artifact.ID, artifact.Path)
		}
	}
	for _, workspace := range manifest.Workspaces {
		full, _, err := resolveWithin(rootAbs, workspace.Path)
		if err != nil {
			return fmt.Errorf("verify workspace %q: %w", workspace.State, err)
		}
		if err := ensureResolvedWithin(rootAbs, full); err != nil {
			return fmt.Errorf("verify workspace %q: %w", workspace.State, err)
		}
		entries, digest, err := snapshotTree(full)
		if err != nil {
			return fmt.Errorf("verify workspace %q: %w", workspace.State, err)
		}
		if digest != workspace.TreeSHA256 || !workspaceEntriesEqual(entries, workspace.Entries) {
			return fmt.Errorf("verify workspace %q: workspace changed", workspace.State)
		}
	}
	if err := verifyRepositoryCurrent(context.Background(), rootAbs, manifest); err != nil {
		return fmt.Errorf("verify repository patch derivation: %w", err)
	}
	return nil
}

// VerifyCurrent is the explicit pipeline-facing name for Verify. It must be
// called immediately before translation, proof, and certificate issuance so
// evidence from a changed artifact, workspace, or executable cannot be reused.
func VerifyCurrent(root string, manifest Manifest) error {
	return Verify(root, manifest)
}

// Validate checks canonical ordering, required workspace semantics, all digest
// formats, and the manifest's self-hash without touching the filesystem.
func Validate(manifest Manifest) error {
	if manifest.Schema != SchemaVersion {
		return fmt.Errorf("manifest schema %q, want %q", manifest.Schema, SchemaVersion)
	}
	normalizedEnvironment, err := validateEnvironment(manifest.Environment)
	if err != nil {
		return err
	}
	if !environmentsEqual(manifest.Environment, normalizedEnvironment) {
		return errors.New("manifest environment is not canonical")
	}
	if len(manifest.Artifacts) == 0 {
		return errors.New("manifest has no artifacts")
	}
	seenArtifactPaths := map[string]bool{}
	for i, artifact := range manifest.Artifacts {
		if artifact.ID == "" || artifact.Kind == "" {
			return errors.New("manifest artifact id and kind are required")
		}
		clean, err := cleanRelative(artifact.Path)
		if err != nil || clean != artifact.Path {
			return fmt.Errorf("manifest artifact %q has non-canonical path %q", artifact.ID, artifact.Path)
		}
		if artifact.Size < 0 || !validDigest(artifact.SHA256) {
			return fmt.Errorf("manifest artifact %q has invalid size or digest", artifact.ID)
		}
		if seenArtifactPaths[artifact.Path] {
			return fmt.Errorf("manifest has duplicate artifact path %q", artifact.Path)
		}
		seenArtifactPaths[artifact.Path] = true
		if i > 0 && manifest.Artifacts[i-1].ID >= artifact.ID {
			return errors.New("manifest artifacts are duplicated or not in canonical order")
		}
	}
	requiredInputs, err := normalizeRequiredInputs("", manifest.RequiredInputs, manifest.Artifacts)
	if err != nil || !requiredInputsEqual(requiredInputs, manifest.RequiredInputs) {
		return fmt.Errorf("manifest required inputs are invalid or non-canonical: %v", err)
	}
	if manifest.Repository == nil {
		return errors.New("manifest repository and exact base commit are required")
	}
	if len(manifest.Workspaces) != 3 {
		return fmt.Errorf("manifest requires 3 workspaces, got %d", len(manifest.Workspaces))
	}
	wantStates := []WorkspaceState{BaseOldTests, BaseNewTests, SolutionNewTests}
	var seenPaths []string
	for i, workspace := range manifest.Workspaces {
		if workspace.State != wantStates[i] {
			return fmt.Errorf("manifest workspace %d is %q, want %q", i, workspace.State, wantStates[i])
		}
		clean, err := cleanRelative(workspace.Path)
		if err != nil || clean != workspace.Path {
			return fmt.Errorf("manifest workspace %q has invalid path %q", workspace.State, workspace.Path)
		}
		for _, previous := range seenPaths {
			if pathsOverlap(previous, workspace.Path) {
				return fmt.Errorf("manifest workspace paths %q and %q overlap", previous, workspace.Path)
			}
		}
		seenPaths = append(seenPaths, workspace.Path)
		if len(workspace.Entries) == 0 || !validDigest(workspace.TreeSHA256) {
			return fmt.Errorf("manifest workspace %q has no entries or invalid tree digest", workspace.State)
		}
		if err := validateWorkspaceEntries(workspace.Entries); err != nil {
			return fmt.Errorf("manifest workspace %q: %w", workspace.State, err)
		}
		derivation, err := normalizeWorkspaceDerivation(workspace.State, workspace.Derivation)
		if err != nil || !workspaceDerivationsEqual(derivation, workspace.Derivation) {
			return fmt.Errorf("manifest workspace %q derivation is invalid or non-canonical: %v", workspace.State, err)
		}
		if err := validateWorkspaceDerivation(workspace.State, derivation, workspace.Entries, manifest.Workspaces[:i], manifest.Artifacts, manifest.RequiredInputs, manifest.Repository); err != nil {
			return fmt.Errorf("manifest workspace %q derivation: %w", workspace.State, err)
		}
		calculated, err := entriesDigest(workspace.Entries)
		if err != nil || calculated != workspace.TreeSHA256 {
			return fmt.Errorf("manifest workspace %q tree digest mismatch", workspace.State)
		}
		cmd, err := normalizeCommand(workspace.Command)
		if err == nil {
			cmd, err = bindCommandShell(cmd, manifest.Environment.Tools)
		}
		if err != nil || !commandsEqual(cmd, workspace.Command) {
			return fmt.Errorf("manifest workspace %q command is invalid or non-canonical: %v", workspace.State, err)
		}
		if !validDigest(workspace.Result.StdoutSHA256) || !validDigest(workspace.Result.StderrSHA256) || !validDigest(workspace.Result.SignalValueSHA256) {
			return fmt.Errorf("manifest workspace %q has invalid result digest", workspace.State)
		}
	}
	if err := validateRepositoryBindings(manifest); err != nil {
		return err
	}
	if !validDigest(manifest.SHA256) {
		return errors.New("manifest has invalid sha256")
	}
	want, err := manifestBodyDigest(manifest)
	if err != nil {
		return err
	}
	if manifest.SHA256 != want {
		return errors.New("manifest sha256 mismatch")
	}
	return nil
}

func ManifestDigest(manifest Manifest) (string, error) {
	if err := Validate(manifest); err != nil {
		return "", err
	}
	return manifest.SHA256, nil
}

func CanonicalJSON(manifest Manifest) ([]byte, error) {
	if err := Validate(manifest); err != nil {
		return nil, err
	}
	return json.Marshal(manifest)
}

func manifestBodyDigest(manifest Manifest) (string, error) {
	manifest.SHA256 = ""
	b, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("encode manifest: %w", err)
	}
	return digestBytes(b), nil
}

func snapshotTree(root string) ([]WorkspaceEntry, string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, "", err
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("%s is not a directory", root)
	}
	var entries []WorkspaceEntry
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		record := WorkspaceEntry{Path: rel, Mode: uint32(info.Mode().Perm())}
		switch {
		case info.Mode().IsRegular():
			record.Kind = "file"
			record.SHA256, record.Size, err = hashFile(path)
		case info.Mode()&os.ModeSymlink != 0:
			record.Kind = "symlink"
			var target string
			target, err = os.Readlink(path)
			record.Size = int64(len(target))
			record.SHA256 = digestBytes([]byte(target))
		default:
			return fmt.Errorf("workspace entry %q has unsupported mode %s", rel, info.Mode())
		}
		if err != nil {
			return err
		}
		entries = append(entries, record)
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if len(entries) == 0 {
		return nil, "", errors.New("workspace has no files")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	digest, err := entriesDigest(entries)
	return entries, digest, err
}

func entriesDigest(entries []WorkspaceEntry) (string, error) {
	b, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return digestBytes(b), nil
}

func validateWorkspaceEntries(entries []WorkspaceEntry) error {
	for i, entry := range entries {
		clean, err := cleanRelative(entry.Path)
		if err != nil || clean != entry.Path {
			return fmt.Errorf("invalid entry path %q", entry.Path)
		}
		if entry.Kind != "file" && entry.Kind != "symlink" {
			return fmt.Errorf("entry %q has invalid kind %q", entry.Path, entry.Kind)
		}
		if entry.Size < 0 || !validDigest(entry.SHA256) {
			return fmt.Errorf("entry %q has invalid size or digest", entry.Path)
		}
		if i > 0 && entries[i-1].Path >= entry.Path {
			return errors.New("workspace entries are duplicated or not in canonical order")
		}
	}
	return nil
}

func copyTree(source, dest string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode().IsRegular():
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
			if err != nil {
				in.Close()
				return err
			}
			_, copyErr := io.Copy(out, in)
			inCloseErr := in.Close()
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if inCloseErr != nil {
				return inCloseErr
			}
			return closeErr
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			return fmt.Errorf("cannot copy unsupported workspace entry %q", rel)
		}
	})
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), n, nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && value == strings.ToLower(value)
}

func resolveWithin(root, path string) (string, string, error) {
	if path == "" {
		return "", "", errors.New("path is required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(rootAbs, filepath.FromSlash(path))
	}
	full, err = filepath.Abs(full)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(rootAbs, full)
	if err != nil {
		return "", "", err
	}
	if rel == "." {
		return full, ".", nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path %q escapes task root", path)
	}
	return full, filepath.ToSlash(filepath.Clean(rel)), nil
}

// ensureResolvedWithin closes the gap left by lexical path checks: a parent
// directory symlink can make an apparently relative path resolve outside the
// task root. Existing artifacts, workspaces, working directories, and signal
// files must remain contained after all symlinks are evaluated.
func ensureResolvedWithin(root, path string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("resolved path %q escapes task root", path)
	}
	return nil
}

func cleanRelative(path string) (string, error) {
	if path == "." {
		return ".", nil
	}
	if path == "" || filepath.IsAbs(path) {
		return "", fmt.Errorf("path %q must be relative", path)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path %q escapes its root", path)
	}
	return clean, nil
}

func expectedPass(state WorkspaceState) (bool, bool) {
	switch state {
	case BaseOldTests, SolutionNewTests:
		return true, true
	case BaseNewTests:
		return false, true
	default:
		return false, false
	}
}

func pathsOverlap(a, b string) bool {
	if a == "." || b == "." || a == b {
		return true
	}
	a = strings.TrimSuffix(filepath.ToSlash(a), "/")
	b = strings.TrimSuffix(filepath.ToSlash(b), "/")
	return strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func exactEnvironment(declared map[string]string) []string {
	keys := make([]string, 0, len(declared)+1)
	for key := range declared {
		keys = append(keys, key)
	}
	if _, declaredPATH := declared["PATH"]; !declaredPATH {
		keys = append(keys, "PATH")
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		value := declared[key]
		out = append(out, key+"="+value)
	}
	return out
}

// EnvironmentDigest hashes the exact, canonically ordered KEY=VALUE vector
// used by a frozen command.
func EnvironmentDigest(declared map[string]string) string {
	type variable struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	entries := exactEnvironment(declared)
	variables := make([]variable, 0, len(entries))
	for _, entry := range entries {
		name, value, _ := strings.Cut(entry, "=")
		variables = append(variables, variable{Name: name, Value: value})
	}
	encoded, _ := json.Marshal(variables)
	return digestBytes(encoded)
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func environmentsEqual(a, b Environment) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ab, bb)
}

func commandsEqual(a, b Command) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ab, bb)
}

func workspaceDerivationsEqual(a, b WorkspaceDerivation) bool {
	if a.Parent != b.Parent || len(a.Changes) != len(b.Changes) {
		return false
	}
	for index := range a.Changes {
		if a.Changes[index] != b.Changes[index] {
			return false
		}
	}
	return true
}

func workspaceEntriesEqual(a, b []WorkspaceEntry) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ab, bb)
}
