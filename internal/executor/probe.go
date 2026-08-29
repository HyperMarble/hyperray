package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// ProbeWorkspace binds a direct probe to the immutable solution+new-tests
// workspace from which its disposable execution copy is made.
type ProbeWorkspace struct {
	ID         string                    `json:"id"`
	Root       string                    `json:"root"`
	State      semanticir.WorkspaceState `json:"state"`
	TreeSHA256 string                    `json:"tree_sha256"`
}

// ProbeHarness is frontend-generated executable materialization of one
// semantic reference witness. Path is workspace-relative. Bytes are staged
// only in a disposable workspace copy; frozen source is never edited.
type ProbeHarness struct {
	Path   string `json:"path"`
	Bytes  []byte `json:"bytes"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

// ProbeStepKind gives ordered probe execution a closed shape. A compile step
// invokes a frozen tool and may produce named generated files. The final run
// step either invokes a frozen tool directly or executes exactly one output
// produced by an earlier compile step.
type ProbeStepKind string

const (
	ProbeStepCompile ProbeStepKind = "compile"
	ProbeStepRun     ProbeStepKind = "run"
)

// ProbeStep is one exact, hermetic command in a direct probe. Tool and
// GeneratedExecutable are mutually exclusive. Argv always includes argv[0]:
// for Tool it must be Tool.Path; for GeneratedExecutable it must be the same
// canonical workspace-relative path. Outputs are canonical workspace-relative
// paths which must not exist before their producing compile step.
type ProbeStep struct {
	ID                  string              `json:"id"`
	Kind                ProbeStepKind       `json:"kind"`
	Tool                *semanticir.ToolRef `json:"tool,omitempty"`
	GeneratedExecutable string              `json:"generated_executable,omitempty"`
	Argv                []string            `json:"argv"`
	WorkDir             string              `json:"work_dir"`
	Environment         []string            `json:"environment,omitempty"`
	Timeout             time.Duration       `json:"timeout"`
	PassSignal          PassSignal          `json:"pass_signal"`
	Outputs             []string            `json:"outputs,omitempty"`
	ObservationPath     string              `json:"observation_path,omitempty"`
}

// ProbeObservation is the only JSON shape a generated harness may emit. It
// contains runtime facts only: operation identity, semantic IDs, provenance,
// and proof-witness structure remain Hyperray-owned plan data.
type ProbeObservation struct {
	Traces []semanticir.RawOutcomeTrace `json:"traces"`
}

// ProbeObservedChoice is derived centrally by pairing one raw trace with the
// corresponding Hyperray-owned behavior and classifying it against the frozen
// operation outcome alphabet.
type ProbeObservedChoice struct {
	Behavior            semanticir.BehaviorRef     `json:"behavior"`
	RawOutcome          semanticir.RawOutcomeTrace `json:"raw_outcome"`
	ClassifiedOutcomeID string                     `json:"classified_outcome_id"`
}

// ProbePlan is an executable direct reference witness. It is produced by a
// language frontend, not inferred by the executor. Every mutable input is
// bound to immutable source, workspace, harness, and tool digests.
type ProbePlan struct {
	ID                string                       `json:"id"`
	WitnessID         string                       `json:"witness_id"`
	Obligation        semanticir.ProofObligation   `json:"obligation"`
	Witness           semanticir.Counterexample    `json:"witness"`
	SourceArtifacts   []semanticir.ArtifactRef     `json:"source_artifacts"`
	Workspace         ProbeWorkspace               `json:"workspace"`
	Tools             []semanticir.ToolRef         `json:"tools"`
	Operations        []semanticir.Operation       `json:"operations"`
	Harness           ProbeHarness                 `json:"harness"`
	Steps             []ProbeStep                  `json:"steps"`
	ExpectedSemantics semanticir.ExpectedSemantics `json:"expected"`
}

// ProbeGeneratedOutputEvidence binds one freshly created compile output to
// its producer. A generated executable is rehashed immediately before and
// after the final run step.
type ProbeGeneratedOutputEvidence struct {
	Path              string `json:"path"`
	ProducerStepID    string `json:"producer_step_id"`
	SHA256            string `json:"sha256"`
	Mode              uint32 `json:"mode"`
	Size              int64  `json:"size"`
	Fresh             bool   `json:"fresh"`
	VerifiedBeforeRun bool   `json:"verified_before_run,omitempty"`
	VerifiedAfterRun  bool   `json:"verified_after_run,omitempty"`
	BeforeRunSHA256   string `json:"before_run_sha256,omitempty"`
	AfterRunSHA256    string `json:"after_run_sha256,omitempty"`
}

// ProbeStepEvidence retains the exact declared step, its canonical digest,
// command observation, and any generated outputs it produced.
type ProbeStepEvidence struct {
	Step       ProbeStep                      `json:"step"`
	StepSHA256 string                         `json:"step_sha256"`
	Command    CommandEvidence                `json:"command"`
	Outputs    []ProbeGeneratedOutputEvidence `json:"outputs,omitempty"`
}

// BindingEvidence records a fresh digest check for one immutable input.
type BindingEvidence struct {
	ID             string `json:"id"`
	Path           string `json:"path"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ObservedSHA256 string `json:"observed_sha256"`
	Version        string `json:"version,omitempty"`
	Verified       bool   `json:"verified"`
}

// ProbeConfirmation is the complete certificate-facing record for one direct
// probe. Plan is retained in full so the certificate can cross-check all
// semantic, workspace, source, tool, harness, and command bindings.
type ProbeConfirmation struct {
	Plan                     ProbePlan                    `json:"plan"`
	PlanSHA256               string                       `json:"plan_sha256"`
	WitnessSHA256            string                       `json:"witness_sha256"`
	WorkspaceSHA256          string                       `json:"workspace_sha256"`
	CopiedWorkspaceSHA256    string                       `json:"copied_workspace_sha256,omitempty"`
	SourceBindings           []BindingEvidence            `json:"source_bindings"`
	ToolBindings             []BindingEvidence            `json:"tool_bindings"`
	ToolsVerifiedAfterRun    bool                         `json:"tools_verified_after_run"`
	HarnessPath              string                       `json:"harness_path"`
	HarnessSHA256            string                       `json:"harness_sha256"`
	StagedHarnessSHA256      string                       `json:"staged_harness_sha256,omitempty"`
	HarnessPreviouslyExisted bool                         `json:"harness_previously_existed"`
	HarnessRestored          bool                         `json:"harness_restored"`
	HarnessRemoved           bool                         `json:"harness_removed"`
	CommandSHA256            string                       `json:"command_sha256"`
	StepsSHA256              string                       `json:"steps_sha256"`
	Steps                    []ProbeStepEvidence          `json:"steps"`
	ObservationPath          string                       `json:"observation_path"`
	FreshObservation         bool                         `json:"fresh_observation"`
	Expected                 semanticir.ExpectedSemantics `json:"expected"`
	ExpectedSHA256           string                       `json:"expected_sha256"`
	Observed                 *ProbeObservation            `json:"observed,omitempty"`
	ObservedSHA256           string                       `json:"observed_sha256,omitempty"`
	Derived                  []ProbeObservedChoice        `json:"derived"`
	DerivedSHA256            string                       `json:"derived_sha256,omitempty"`
	SemanticsMatch           bool                         `json:"semantics_match"`
	IsolatedWorkspaceRoot    string                       `json:"isolated_workspace_root"`
	IsolatedWorkspaceRemoved bool                         `json:"isolated_workspace_removed"`
	OriginalWorkspaceIntact  bool                         `json:"original_workspace_intact"`
	Error                    string                       `json:"error,omitempty"`
}

// ProbeEvidence is retained as the field-level name used by Confirmation.
// It aliases the certificate-facing ProbeConfirmation record exactly.
type ProbeEvidence = ProbeConfirmation

type preparedProbe struct {
	plan            ProbePlan
	root            string
	harnessPath     string
	observationPath string
	steps           []preparedProbeStep
	sources         []preparedBinding
	tools           []preparedBinding
}

type preparedProbeStep struct {
	step                ProbeStep
	workDir             string
	outputs             []string
	generatedExecutable string
}

type preparedBinding struct {
	ref     semanticir.ArtifactRef
	tool    semanticir.ToolRef
	path    string
	digest  string
	version string
}

type localFileSnapshot struct {
	existed bool
	body    []byte
	mode    os.FileMode
}

type probeWorkspaceEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// ConfirmProbes confirms frontend-generated direct reference probes. The
// baseline verifier is mandatory and runs in its own disposable copy. Each
// probe then runs in a new copy, so neither harness staging nor a probe process
// can turn the clean baseline into proof evidence or modify frozen source.
func ConfirmProbes(ctx context.Context, baseline TaskEnvironment, plans []ProbePlan) (report Report) {
	report.Status = StatusProofBlocked
	report.Vacuous = len(plans) == 0
	if len(plans) == 0 {
		return Confirm(ctx, baseline, nil)
	}
	if ctx == nil {
		report.Blockers = append(report.Blockers, Blocker{Stage: "probe", Code: "nil-context", Detail: "execution context is nil"})
		return report
	}

	prepared, blockers := prepareProbePlans(plans)
	if len(blockers) != 0 {
		report.Blockers = append(report.Blockers, blockers...)
		return report
	}
	defer func() { report.Status = aggregateStatus(report) }()

	baselineEvidence, baselineIsolation, blocker := runIsolatedProbeBaseline(ctx, baseline, prepared[0])
	report.Baseline, report.BaselineIsolation = baselineEvidence, baselineIsolation
	if blocker != nil {
		report.Blockers = append(report.Blockers, *blocker)
		return report
	}
	if !report.Baseline.Passed {
		report.Blockers = append(report.Blockers, Blocker{
			Stage: "baseline", Code: "baseline-failed",
			Detail: "the unmodified reference did not produce the declared pass signal",
		})
		return report
	}

	for _, plan := range prepared {
		confirmation, safe := confirmProbe(ctx, plan)
		report.Confirmations = append(report.Confirmations, confirmation)
		report.Blockers = append(report.Blockers, confirmation.Blockers...)
		if !safe {
			break
		}
	}
	return report
}

func prepareProbePlans(plans []ProbePlan) ([]preparedProbe, []Blocker) {
	prepared := make([]preparedProbe, 0, len(plans))
	planIDs := make(map[string]bool, len(plans))
	witnessIDs := make(map[string]bool, len(plans))
	for _, plan := range plans {
		probe, blocker := prepareProbePlan(plan)
		if blocker != nil {
			return nil, []Blocker{*blocker}
		}
		if planIDs[plan.ID] || witnessIDs[plan.WitnessID] {
			return nil, []Blocker{{
				Stage: "probe", PlanID: plan.ID, WitnessID: plan.WitnessID,
				Code: "duplicate-probe", Detail: "probe plan and witness IDs must be unique",
			}}
		}
		planIDs[plan.ID], witnessIDs[plan.WitnessID] = true, true
		if len(prepared) > 0 && (probe.root != prepared[0].root || plan.Workspace.TreeSHA256 != prepared[0].plan.Workspace.TreeSHA256) {
			return nil, []Blocker{{
				Stage: "probe", PlanID: plan.ID, WitnessID: plan.WitnessID,
				Code:   "inconsistent-probe-workspace",
				Detail: "one confirmation report must use one frozen solution+new-tests workspace",
			}}
		}
		prepared = append(prepared, probe)
	}
	return prepared, nil
}

func prepareProbePlan(plan ProbePlan) (preparedProbe, *Blocker) {
	block := func(code, detail string) (preparedProbe, *Blocker) {
		return preparedProbe{}, &Blocker{Stage: "probe", PlanID: plan.ID, WitnessID: plan.WitnessID, Code: code, Detail: detail}
	}
	if plan.ID == "" || plan.WitnessID == "" || plan.Witness.ID == "" || plan.WitnessID != plan.Witness.ID {
		return block("unbound-probe", "probe, witness, and embedded counterexample IDs must be non-empty and identical")
	}
	if plan.Obligation != semanticir.ObligationReferenceCorrectness || plan.Witness.Obligation != plan.Obligation {
		return block("invalid-probe-obligation", "direct probes are only valid for reference-correctness witnesses")
	}
	if plan.Witness.OperationID == "" || len(plan.Witness.Choices) == 0 || len(plan.Witness.ObservedOutcomes) == 0 {
		return block("incomplete-probe-witness", "reference probe witness lacks operation, choices, or observed outcomes")
	}
	if !validDigest(plan.Witness.Provenance.ArtifactDigest) {
		return block("invalid-probe-provenance", "probe witness has no normalized frozen provenance digest")
	}
	if err := validateProbeSemantics(plan.ExpectedSemantics, plan.Witness, plan.Operations); err != nil {
		return block("invalid-probe-semantics", err.Error())
	}

	if plan.Workspace.ID == "" || plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || !validDigest(plan.Workspace.TreeSHA256) {
		return block("invalid-probe-workspace", "probe must bind a solution+new-tests workspace and normalized tree digest")
	}
	if !filepath.IsAbs(plan.Workspace.Root) || filepath.Clean(plan.Workspace.Root) != plan.Workspace.Root {
		return block("invalid-probe-workspace", "probe workspace root must be an absolute canonical path")
	}
	root, err := filepath.Abs(plan.Workspace.Root)
	if err != nil {
		return block("invalid-probe-workspace", err.Error())
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return block("invalid-probe-workspace", fmt.Sprintf("resolving workspace: %v", err))
	}
	root = filepath.Clean(root)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return block("invalid-probe-workspace", fmt.Sprintf("workspace %q is not a readable directory", root))
	}
	observedTree, err := WorkspaceDigest(root)
	if err != nil {
		return block("invalid-probe-workspace", err.Error())
	}
	if observedTree != plan.Workspace.TreeSHA256 {
		return block("stale-probe-workspace", fmt.Sprintf("workspace digest is %s, plan requires %s", observedTree, plan.Workspace.TreeSHA256))
	}

	if len(plan.SourceArtifacts) == 0 {
		return block("missing-probe-sources", "probe declares no frozen source artifacts")
	}
	sources := make([]preparedBinding, 0, len(plan.SourceArtifacts))
	sourceIDs, sourcePaths := map[string]bool{}, map[string]bool{}
	for _, ref := range plan.SourceArtifacts {
		if ref.ID == "" || ref.Path == "" || !validDigest(ref.Digest) {
			return block("invalid-probe-source", "probe source has incomplete immutable identity")
		}
		path, err := resolveProbeExisting(root, ref.Path, false)
		if err != nil {
			return block("probe-source-path", err.Error())
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return block("probe-source-unreadable", err.Error())
		}
		observed := digestBytes(body)
		if observed != ref.Digest {
			return block("stale-probe-source", fmt.Sprintf("source %q digest is %s, want %s", ref.ID, observed, ref.Digest))
		}
		if sourceIDs[ref.ID] || sourcePaths[path] {
			return block("duplicate-probe-source", "probe source IDs and resolved paths must be unique")
		}
		sourceIDs[ref.ID], sourcePaths[path] = true, true
		sources = append(sources, preparedBinding{ref: ref, path: path, digest: observed})
	}
	witnessSourceBound := false
	for _, source := range sources {
		if source.ref.ID == plan.Witness.Provenance.ArtifactID && source.ref.Digest == plan.Witness.Provenance.ArtifactDigest {
			witnessSourceBound = true
			break
		}
	}
	if !witnessSourceBound {
		return block("unbound-probe-source", "witness provenance is not bound by a declared frozen source artifact")
	}

	if len(plan.Tools) == 0 {
		return block("missing-probe-tools", "probe declares no frozen tool bindings")
	}
	tools := make([]preparedBinding, 0, len(plan.Tools))
	toolNames, toolPaths := map[string]bool{}, map[string]bool{}
	for _, tool := range plan.Tools {
		if tool.Name == "" || tool.Path == "" || tool.Version == "" || !validDigest(tool.Digest) || !filepath.IsAbs(tool.Path) {
			return block("invalid-probe-tool", "probe tool has incomplete immutable identity")
		}
		path, err := filepath.EvalSymlinks(tool.Path)
		if err != nil {
			return block("probe-tool-unreadable", err.Error())
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			return block("probe-tool-unreadable", fmt.Sprintf("tool %q is not a regular executable", tool.Name))
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return block("probe-tool-unreadable", err.Error())
		}
		observed := digestBytes(body)
		if observed != tool.Digest {
			return block("stale-probe-tool", fmt.Sprintf("tool %q digest is %s, want %s", tool.Name, observed, tool.Digest))
		}
		path = filepath.Clean(path)
		if toolNames[tool.Name] || toolPaths[path] {
			return block("duplicate-probe-tool", "probe tool names and resolved paths must be unique")
		}
		toolNames[tool.Name], toolPaths[path] = true, true
		tools = append(tools, preparedBinding{tool: tool, path: path, digest: observed, version: tool.Version})
	}

	if len(plan.Harness.Bytes) == 0 || !validDigest(plan.Harness.SHA256) || digestBytes(plan.Harness.Bytes) != plan.Harness.SHA256 {
		return block("invalid-probe-harness", "probe harness bytes do not match a normalized declared digest")
	}
	if plan.Harness.Mode == 0 || plan.Harness.Mode&^uint32(0o777) != 0 {
		return block("invalid-probe-harness", "probe harness mode must contain only non-zero permission bits")
	}
	harnessRelative, err := cleanProbeRelative(plan.Harness.Path)
	if err != nil {
		return block("probe-harness-path", err.Error())
	}
	if plan.Harness.Path != filepath.ToSlash(harnessRelative) {
		return block("probe-harness-path", "probe harness path is not canonical")
	}
	if _, err := resolveProbeParent(root, harnessRelative); err != nil {
		return block("probe-harness-path", err.Error())
	}

	steps, observation, err := validateProbeSteps(root, plan.Steps, tools)
	if err != nil {
		return block("invalid-probe-steps", err.Error())
	}
	if harnessRelative == observation {
		return block("invalid-probe-paths", "harness and observation paths must be distinct")
	}
	generatedPaths := make(map[string]bool)
	for _, step := range steps {
		for _, output := range step.outputs {
			generatedPaths[output] = true
		}
	}
	for _, step := range steps {
		if step.step.PassSignal.VerdictFile != nil {
			verdictRelative := filepath.Clean(filepath.FromSlash(step.step.PassSignal.VerdictFile.Path))
			if harnessRelative == verdictRelative || observation == verdictRelative || generatedPaths[verdictRelative] {
				return block("invalid-probe-paths", "harness, observation, generated output, and verdict paths must be distinct")
			}
		}
		for _, output := range step.outputs {
			if harnessRelative == output || observation == output {
				return block("invalid-probe-paths", "harness, observation, and generated output paths must be distinct")
			}
		}
	}
	return preparedProbe{
		plan: plan, root: root, harnessPath: harnessRelative,
		observationPath: observation, steps: steps, sources: sources, tools: tools,
	}, nil
}

func validateProbeSemantics(expected semanticir.ExpectedSemantics, witness semanticir.Counterexample, operations []semanticir.Operation) error {
	if !reflect.DeepEqual(expected.Conditions, witness.Conditions) || expected.OperationID != witness.OperationID || !reflect.DeepEqual(expected.OutcomeIDs, witness.ObservedOutcomes) || !reflect.DeepEqual(expected.Choices, witness.Choices) || expected.TestPasses != witness.TestPasses {
		return fmt.Errorf("expected semantics do not preserve the complete Hyperray-owned proof witness")
	}
	if len(witness.Choices) == 0 || len(witness.ObservedOutcomes) != len(witness.Choices) || len(expected.RuntimeOutcomes) != len(witness.Choices) {
		return fmt.Errorf("expected runtime outcome vector is incomplete")
	}
	operationByID := make(map[string]semanticir.Operation, len(operations))
	for _, operation := range operations {
		if operation.ID == "" || operationByID[operation.ID].ID != "" || len(operation.OutcomeIDs) == 0 {
			return fmt.Errorf("probe operations have incomplete or duplicate outcome alphabets")
		}
		seen := map[string]bool{}
		for _, outcomeID := range operation.OutcomeIDs {
			if outcomeID == "" || seen[outcomeID] {
				return fmt.Errorf("probe operation %q has an empty or duplicate outcome ID", operation.ID)
			}
			seen[outcomeID] = true
		}
		operationByID[operation.ID] = operation
	}
	for index, choice := range witness.Choices {
		runtime := expected.RuntimeOutcomes[index]
		operation, exists := operationByID[choice.Behavior.OperationID]
		classified, err := semanticir.ClassifyRawOutcome(operation, runtime.RawOutcome, choice.Behavior.Provenance)
		if !exists || err != nil || choice.Behavior.OperationID == "" || choice.OutcomeID == "" || witness.ObservedOutcomes[index] != choice.OutcomeID || !reflect.DeepEqual(runtime.Behavior, choice.Behavior) || runtime.MappingOutcomeID != classified || runtime.MappingOutcomeID != choice.OutcomeID {
			return fmt.Errorf("runtime outcome %d is not centrally classified as its Hyperray-owned behavior choice", index)
		}
	}
	return nil
}

func validateProbeSteps(root string, declared []ProbeStep, tools []preparedBinding) ([]preparedProbeStep, string, error) {
	if len(declared) == 0 {
		return nil, "", fmt.Errorf("probe declares no ordered execution steps")
	}
	steps := make([]preparedProbeStep, 0, len(declared))
	stepIDs := make(map[string]bool, len(declared))
	generated := make(map[string]bool)
	for index, step := range declared {
		if step.ID == "" || stepIDs[step.ID] {
			return nil, "", fmt.Errorf("probe step %d has an empty or duplicate ID", index)
		}
		stepIDs[step.ID] = true
		final := index == len(declared)-1
		if final && step.Kind != ProbeStepRun {
			return nil, "", fmt.Errorf("final probe step must be run")
		}
		if !final && step.Kind != ProbeStepCompile {
			return nil, "", fmt.Errorf("every non-final probe step must be compile")
		}
		if len(step.Argv) == 0 || step.Argv[0] == "" {
			return nil, "", fmt.Errorf("probe step %q has empty argv", step.ID)
		}
		for _, argument := range step.Argv {
			if strings.ContainsRune(argument, '\x00') || !utf8.ValidString(argument) {
				return nil, "", fmt.Errorf("probe step %q argv contains non-canonical text", step.ID)
			}
		}
		if step.Timeout <= 0 {
			return nil, "", fmt.Errorf("probe step %q timeout must be greater than zero", step.ID)
		}
		if (step.PassSignal.ExitCode == nil) == (step.PassSignal.VerdictFile == nil) {
			return nil, "", fmt.Errorf("probe step %q must declare exactly one pass signal", step.ID)
		}
		environmentNames := make(map[string]bool, len(step.Environment))
		for _, entry := range step.Environment {
			name, _, ok := strings.Cut(entry, "=")
			if !ok || name == "" || strings.ContainsRune(entry, '\x00') || !utf8.ValidString(entry) || environmentNames[name] {
				return nil, "", fmt.Errorf("probe step %q has invalid environment entry %q", step.ID, entry)
			}
			environmentNames[name] = true
		}
		workRelative, err := cleanProbeRelativeAllowDot(step.WorkDir)
		if err != nil || step.WorkDir != filepath.ToSlash(workRelative) {
			return nil, "", fmt.Errorf("probe step %q has a non-canonical workdir", step.ID)
		}
		workDir, err := resolveProbeExisting(root, workRelative, true)
		if err != nil {
			return nil, "", fmt.Errorf("probe step %q workdir: %w", step.ID, err)
		}

		toolMode := step.Tool != nil
		generatedMode := step.GeneratedExecutable != ""
		if toolMode == generatedMode {
			return nil, "", fmt.Errorf("probe step %q must declare exactly one frozen tool or generated executable", step.ID)
		}
		prepared := preparedProbeStep{step: step, workDir: workDir}
		if toolMode {
			if step.Argv[0] != step.Tool.Path || !filepath.IsAbs(step.Argv[0]) {
				return nil, "", fmt.Errorf("probe step %q argv[0] differs from its frozen tool", step.ID)
			}
			resolvedTool, err := filepath.EvalSymlinks(step.Tool.Path)
			if err != nil {
				return nil, "", fmt.Errorf("probe step %q tool: %w", step.ID, err)
			}
			bound := false
			for _, tool := range tools {
				if reflect.DeepEqual(*step.Tool, tool.tool) && filepath.Clean(resolvedTool) == tool.path {
					bound = true
					break
				}
			}
			if !bound {
				return nil, "", fmt.Errorf("probe step %q tool is not an exact member of Tools", step.ID)
			}
		} else {
			if step.Kind != ProbeStepRun {
				return nil, "", fmt.Errorf("compile step %q cannot execute a generated output", step.ID)
			}
			executable, err := cleanProbeRelative(step.GeneratedExecutable)
			if err != nil || step.GeneratedExecutable != filepath.ToSlash(executable) || step.Argv[0] != step.GeneratedExecutable {
				return nil, "", fmt.Errorf("run step %q has a non-canonical generated executable reference", step.ID)
			}
			if !generated[executable] {
				return nil, "", fmt.Errorf("run step %q does not reference an exact prior compile output", step.ID)
			}
			prepared.generatedExecutable = executable
		}

		if step.Kind == ProbeStepCompile {
			if !toolMode || len(step.Outputs) == 0 || step.ObservationPath != "" {
				return nil, "", fmt.Errorf("compile step %q must use a frozen tool, declare outputs, and omit observation", step.ID)
			}
			for _, declaredOutput := range step.Outputs {
				output, err := cleanProbeRelative(declaredOutput)
				if err != nil || declaredOutput != filepath.ToSlash(output) || generated[output] {
					return nil, "", fmt.Errorf("compile step %q has a non-canonical or duplicate output %q", step.ID, declaredOutput)
				}
				if _, err := resolveProbeParent(root, output); err != nil {
					return nil, "", fmt.Errorf("compile step %q output: %w", step.ID, err)
				}
				generated[output] = true
				prepared.outputs = append(prepared.outputs, output)
			}
		} else if len(step.Outputs) != 0 {
			return nil, "", fmt.Errorf("run step %q cannot declare generated outputs", step.ID)
		}
		if verdict := step.PassSignal.VerdictFile; verdict != nil {
			path, err := cleanProbeRelative(verdict.Path)
			if err != nil || verdict.Path != filepath.ToSlash(path) || strings.TrimSpace(verdict.PassValue) == "" || !utf8.ValidString(verdict.PassValue) {
				return nil, "", fmt.Errorf("probe step %q has an invalid verdict-file signal", step.ID)
			}
			if _, err := resolveProbeParent(root, path); err != nil {
				return nil, "", fmt.Errorf("probe step %q verdict: %w", step.ID, err)
			}
		}
		steps = append(steps, prepared)
	}
	final := &steps[len(steps)-1]
	observation, err := cleanProbeRelative(final.step.ObservationPath)
	if err != nil || final.step.ObservationPath != filepath.ToSlash(observation) {
		return nil, "", fmt.Errorf("final run step has a non-canonical observation path")
	}
	if _, err := resolveProbeParent(root, observation); err != nil {
		return nil, "", fmt.Errorf("final run observation: %w", err)
	}
	for output := range generated {
		if output == observation {
			return nil, "", fmt.Errorf("generated output and observation paths must be distinct")
		}
	}
	return steps, observation, nil
}

func runIsolatedProbeBaseline(ctx context.Context, baseline TaskEnvironment, probe preparedProbe) (CommandEvidence, *IsolationEvidence, *Blocker) {
	resolvedWorkDir, resolvedWorkspaceRoot, configBlocker := validateTask(baseline)
	if configBlocker != nil {
		configBlocker.Stage = "baseline"
		return CommandEvidence{}, nil, configBlocker
	}
	if resolvedWorkspaceRoot != probe.root || !pathWithin(probe.root, resolvedWorkDir) {
		return CommandEvidence{}, nil, &Blocker{Stage: "baseline", Code: "baseline-outside-workspace", Detail: "baseline workspace root or workdir does not match the probe workspace"}
	}
	baseline.WorkDir, baseline.WorkspaceRoot = resolvedWorkDir, resolvedWorkspaceRoot
	baseline.WorkspaceSHA256 = probe.plan.Workspace.TreeSHA256
	return runIsolatedBaseline(ctx, baseline)
}

func confirmProbe(ctx context.Context, prepared preparedProbe) (confirmation Confirmation, safe bool) {
	plan := prepared.plan
	confirmation = Confirmation{
		PlanIDs: []string{plan.ID}, WitnessID: plan.WitnessID, Mode: ConfirmationModeProbe,
		Status: StatusProofBlocked,
	}
	planDigest, _ := semanticir.Digest(plan)
	witnessDigest, _ := semanticir.Digest(plan.Witness)
	expectedDigest, _ := semanticir.Digest(plan.ExpectedSemantics)
	stepsDigest, _ := semanticir.Digest(plan.Steps)
	commandDigest, _ := semanticir.Digest(plan.Steps[len(plan.Steps)-1])
	evidence := &ProbeEvidence{
		Plan: plan, PlanSHA256: planDigest, WitnessSHA256: witnessDigest,
		WorkspaceSHA256: plan.Workspace.TreeSHA256,
		HarnessPath:     plan.Harness.Path, HarnessSHA256: plan.Harness.SHA256,
		CommandSHA256: commandDigest, StepsSHA256: stepsDigest, ObservationPath: prepared.observationPath,
		Expected: plan.ExpectedSemantics, ExpectedSHA256: expectedDigest,
	}
	confirmation.Probe = evidence
	block := func(code, detail string) {
		confirmation.Blockers = append(confirmation.Blockers, Blocker{
			Stage: "probe", PlanID: plan.ID, WitnessID: plan.WitnessID, Code: code, Detail: detail,
		})
	}
	for _, source := range prepared.sources {
		evidence.SourceBindings = append(evidence.SourceBindings, BindingEvidence{
			ID: source.ref.ID, Path: source.path, ExpectedSHA256: source.ref.Digest,
			ObservedSHA256: source.digest, Verified: source.digest == source.ref.Digest,
		})
	}
	for _, tool := range prepared.tools {
		evidence.ToolBindings = append(evidence.ToolBindings, BindingEvidence{
			ID: tool.tool.Name, Path: tool.path, ExpectedSHA256: tool.tool.Digest,
			ObservedSHA256: tool.digest, Version: tool.version, Verified: tool.digest == tool.tool.Digest,
		})
	}

	tempParent, runRoot, copiedDigest, err := makeProbeWorkspaceCopy(prepared.root, plan.Workspace.TreeSHA256)
	if err != nil {
		evidence.Error = err.Error()
		block("probe-copy-failed", err.Error())
		return confirmation, false
	}
	evidence.CopiedWorkspaceSHA256 = copiedDigest
	evidence.IsolatedWorkspaceRoot = runRoot
	safe = true
	harnessPath := filepath.Join(runRoot, filepath.FromSlash(plan.Harness.Path))
	observationPath := filepath.Join(runRoot, filepath.FromSlash(prepared.observationPath))
	harnessSnapshot, err := snapshotLocalFile(harnessPath)
	if err != nil {
		evidence.Error = err.Error()
		block("probe-harness-snapshot", err.Error())
	}
	observationSnapshot, observationErr := snapshotLocalFile(observationPath)
	if observationErr != nil {
		evidence.Error = appendError(evidence.Error, observationErr.Error())
		block("probe-observation-snapshot", observationErr.Error())
	}
	if err != nil || observationErr != nil {
		if removeErr := os.RemoveAll(tempParent); removeErr != nil {
			safe = false
			block("probe-workspace-cleanup", removeErr.Error())
		}
		return confirmation, safe
	}
	evidence.HarnessPreviouslyExisted = harnessSnapshot.existed

	defer func() {
		if verifyErr := verifyPreparedProbeTools(prepared.tools); verifyErr != nil {
			evidence.Error = appendError(evidence.Error, "verify tools after probe: "+verifyErr.Error())
			block("probe-tool-changed", verifyErr.Error())
			confirmation.Status = StatusProofBlocked
			safe = false
		} else {
			evidence.ToolsVerifiedAfterRun = true
		}
		if restoreErr := restoreLocalFile(harnessPath, harnessSnapshot); restoreErr != nil {
			evidence.Error = appendError(evidence.Error, "restore harness: "+restoreErr.Error())
			block("probe-harness-restore", restoreErr.Error())
			confirmation.Status = StatusProofBlocked
			safe = false
		} else if harnessSnapshot.existed {
			evidence.HarnessRestored = localFileEquals(harnessPath, harnessSnapshot.body)
			if !evidence.HarnessRestored {
				block("probe-harness-restore", "restored harness bytes do not match isolated workspace snapshot")
				confirmation.Status = StatusProofBlocked
				safe = false
			}
		} else {
			_, statErr := os.Lstat(harnessPath)
			evidence.HarnessRemoved = os.IsNotExist(statErr)
			if !evidence.HarnessRemoved {
				block("probe-harness-remove", "generated harness remains in isolated workspace")
				confirmation.Status = StatusProofBlocked
				safe = false
			}
		}
		if restoreErr := restoreLocalFile(observationPath, observationSnapshot); restoreErr != nil {
			evidence.Error = appendError(evidence.Error, "restore observation: "+restoreErr.Error())
			block("probe-observation-restore", restoreErr.Error())
			confirmation.Status = StatusProofBlocked
			safe = false
		}
		if removeErr := os.RemoveAll(tempParent); removeErr != nil {
			evidence.Error = appendError(evidence.Error, "remove isolated workspace: "+removeErr.Error())
			block("probe-workspace-cleanup", removeErr.Error())
			confirmation.Status = StatusProofBlocked
			safe = false
		} else {
			_, statErr := os.Stat(tempParent)
			evidence.IsolatedWorkspaceRemoved = os.IsNotExist(statErr)
			if !evidence.IsolatedWorkspaceRemoved {
				block("probe-workspace-cleanup", "isolated workspace still exists after cleanup")
				confirmation.Status = StatusProofBlocked
				safe = false
			}
		}
		currentDigest, digestErr := WorkspaceDigest(prepared.root)
		evidence.OriginalWorkspaceIntact = digestErr == nil && currentDigest == plan.Workspace.TreeSHA256
		if !evidence.OriginalWorkspaceIntact {
			detail := "original frozen workspace changed during isolated probe"
			if digestErr != nil {
				detail += ": " + digestErr.Error()
			}
			block("probe-workspace-mutated", detail)
			confirmation.Status = StatusProofBlocked
			safe = false
		}
	}()

	if verifyErr := verifyPreparedProbeTools(prepared.tools); verifyErr != nil {
		evidence.Error = "verify tools before probe: " + verifyErr.Error()
		block("stale-probe-tool", verifyErr.Error())
		return confirmation, false
	}
	generatedParents := []string{filepath.Dir(harnessPath), filepath.Dir(observationPath)}
	for _, step := range prepared.steps {
		if step.step.PassSignal.VerdictFile != nil {
			generatedParents = append(generatedParents, filepath.Dir(filepath.Join(runRoot, filepath.FromSlash(step.step.PassSignal.VerdictFile.Path))))
		}
		for _, output := range step.outputs {
			generatedParents = append(generatedParents, filepath.Dir(filepath.Join(runRoot, filepath.FromSlash(output))))
		}
	}
	for _, parent := range generatedParents {
		if err := os.MkdirAll(parent, 0o750); err != nil {
			evidence.Error = "create generated probe directory: " + err.Error()
			block("probe-directory-stage", err.Error())
			return confirmation, false
		}
	}

	if err := writeExact(harnessPath, plan.Harness.Bytes, os.FileMode(plan.Harness.Mode)); err != nil {
		evidence.Error = "stage harness: " + err.Error()
		block("probe-harness-stage", err.Error())
		return confirmation, false
	}
	staged, err := os.ReadFile(harnessPath)
	if err != nil {
		evidence.Error = "read staged harness: " + err.Error()
		block("probe-harness-stage", err.Error())
		return confirmation, false
	}
	evidence.StagedHarnessSHA256 = digestBytes(staged)
	if evidence.StagedHarnessSHA256 != plan.Harness.SHA256 {
		evidence.Error = "staged harness digest mismatch"
		block("probe-harness-stage", evidence.Error)
		return confirmation, false
	}
	if err := os.Remove(observationPath); err != nil && !os.IsNotExist(err) {
		evidence.Error = "clear stale observation: " + err.Error()
		block("probe-observation-clear", err.Error())
		return confirmation, false
	}

	for stepIndex, preparedStep := range prepared.steps {
		step := preparedStep.step
		stepRecord := ProbeStepEvidence{Step: step}
		stepRecord.StepSHA256, _ = semanticir.Digest(step)
		// Setup/compile processes are untrusted too. Clear the observation
		// immediately before the sole run step so only that execution can
		// materialize the runtime trace used as confirmation evidence.
		if step.Kind == ProbeStepRun {
			if err := os.Remove(observationPath); err != nil && !os.IsNotExist(err) {
				evidence.Error = appendError(evidence.Error, "clear pre-run observation: "+err.Error())
				block("probe-observation-clear", err.Error())
				return confirmation, false
			}
		}
		for _, output := range preparedStep.outputs {
			outputPath := filepath.Join(runRoot, filepath.FromSlash(output))
			if _, statErr := os.Lstat(outputPath); !os.IsNotExist(statErr) {
				detail := fmt.Sprintf("compile output %q was not fresh before step %q", output, step.ID)
				evidence.Error = appendError(evidence.Error, detail)
				block("probe-output-not-fresh", detail)
				return confirmation, false
			}
		}
		if preparedStep.generatedExecutable != "" {
			if err := verifyGeneratedProbeOutput(runRoot, evidence, preparedStep.generatedExecutable, true); err != nil {
				evidence.Error = appendError(evidence.Error, err.Error())
				block("probe-generated-executable-stale", err.Error())
				return confirmation, false
			}
		}
		relativeWork, _ := filepath.Rel(prepared.root, preparedStep.workDir)
		argv := append([]string(nil), step.Argv...)
		if preparedStep.generatedExecutable != "" {
			argv[0] = filepath.Join(runRoot, filepath.FromSlash(preparedStep.generatedExecutable))
		}
		task := TaskEnvironment{
			Command: argv, WorkspaceRoot: runRoot, WorkDir: filepath.Join(runRoot, relativeWork),
			Environment: append([]string(nil), step.Environment...), Timeout: step.Timeout,
			ExactEnvironment: true, PassSignal: step.PassSignal,
		}
		if task.PassSignal.VerdictFile != nil {
			task.PassSignal = VerdictFileSignal(
				filepath.Join(runRoot, filepath.FromSlash(step.PassSignal.VerdictFile.Path)),
				step.PassSignal.VerdictFile.PassValue,
			)
		}
		stepRecord.Command = runVerifier(ctx, task)
		evidence.Steps = append(evidence.Steps, stepRecord)
		if step.Kind == ProbeStepRun {
			confirmation.Command = stepRecord.Command
		}
		if stepRecord.Command.Error != "" {
			evidence.Error = appendError(evidence.Error, fmt.Sprintf("step %q: %s", step.ID, stepRecord.Command.Error))
			block(commandBlockerCode(stepRecord.Command), fmt.Sprintf("probe step %q: %s", step.ID, stepRecord.Command.Error))
			return confirmation, false
		}
		if stepRecord.Command.OutputTruncated || !utf8.ValidString(stepRecord.Command.Stdout) || !utf8.ValidString(stepRecord.Command.Stderr) {
			detail := fmt.Sprintf("probe step %q output is truncated or not canonical UTF-8", step.ID)
			evidence.Error = appendError(evidence.Error, detail)
			block("probe-output-invalid", detail)
			return confirmation, false
		}
		if !stepRecord.Command.Passed {
			if step.Kind == ProbeStepCompile {
				detail := fmt.Sprintf("compile step %q did not produce its declared pass signal", step.ID)
				evidence.Error = appendError(evidence.Error, detail)
				block("probe-compile-failed", detail)
				return confirmation, false
			}
			confirmation.Command = stepRecord.Command
			detail := fmt.Sprintf("reference probe run step %q did not produce its declared success signal", step.ID)
			evidence.Error = appendError(evidence.Error, detail)
			block("model-execution-mismatch", detail)
			confirmation.Status = StatusProofBlocked
			return confirmation, false
		}
		if step.Kind == ProbeStepCompile {
			outputs := make([]ProbeGeneratedOutputEvidence, 0, len(preparedStep.outputs))
			for _, output := range preparedStep.outputs {
				record, outputErr := inspectGeneratedProbeOutput(runRoot, output, step.ID)
				if outputErr != nil {
					evidence.Error = appendError(evidence.Error, outputErr.Error())
					block("probe-output-invalid", outputErr.Error())
					return confirmation, false
				}
				outputs = append(outputs, record)
			}
			evidence.Steps[stepIndex].Outputs = outputs
		}
		if preparedStep.generatedExecutable != "" {
			if err := verifyGeneratedProbeOutput(runRoot, evidence, preparedStep.generatedExecutable, false); err != nil {
				evidence.Error = appendError(evidence.Error, err.Error())
				block("probe-generated-executable-changed", err.Error())
				return confirmation, false
			}
		}
	}

	observed, derived, observationErr := readProbeObservation(observationPath, plan)
	if observationErr == nil {
		evidence.FreshObservation = true
		evidence.Observed = &observed
		evidence.ObservedSHA256, _ = semanticir.Digest(observed)
		evidence.Derived = derived
		evidence.DerivedSHA256, _ = semanticir.Digest(derived)
		evidence.SemanticsMatch = probeObservationMatchesPlan(observed, derived, plan)
	} else if confirmation.Command.Passed {
		evidence.Error = appendError(evidence.Error, observationErr.Error())
		block("probe-observation-invalid", observationErr.Error())
		return confirmation, false
	}
	if confirmation.Command.Passed && evidence.FreshObservation && evidence.SemanticsMatch {
		confirmation.Status = StatusConfirmed
	} else {
		detail := "observed reference behavior differs from the complete semantic witness vector"
		evidence.Error = appendError(evidence.Error, detail)
		block("model-execution-mismatch", detail)
		confirmation.Status = StatusProofBlocked
	}
	return confirmation, safe
}

func inspectGeneratedProbeOutput(runRoot, relative, producer string) (ProbeGeneratedOutputEvidence, error) {
	path := filepath.Join(runRoot, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if err != nil {
		return ProbeGeneratedOutputEvidence{}, fmt.Errorf("inspect compile output %q: %w", relative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ProbeGeneratedOutputEvidence{}, fmt.Errorf("compile output %q is not a regular non-symlink file", relative)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ProbeGeneratedOutputEvidence{}, fmt.Errorf("read compile output %q: %w", relative, err)
	}
	return ProbeGeneratedOutputEvidence{
		Path: relative, ProducerStepID: producer, SHA256: digestBytes(body),
		Mode: uint32(info.Mode().Perm()), Size: int64(len(body)), Fresh: true,
	}, nil
}

func verifyGeneratedProbeOutput(runRoot string, evidence *ProbeEvidence, relative string, before bool) error {
	for stepIndex := range evidence.Steps {
		for outputIndex := range evidence.Steps[stepIndex].Outputs {
			record := &evidence.Steps[stepIndex].Outputs[outputIndex]
			if record.Path != relative {
				continue
			}
			observed, err := inspectGeneratedProbeOutput(runRoot, relative, record.ProducerStepID)
			if err != nil {
				return err
			}
			if observed.SHA256 != record.SHA256 || observed.Mode != record.Mode || observed.Size != record.Size || !record.Fresh {
				return fmt.Errorf("generated executable %q changed after compile step %q", relative, record.ProducerStepID)
			}
			if observed.Mode&0o111 == 0 {
				return fmt.Errorf("generated executable %q has no execute permission", relative)
			}
			if before {
				record.VerifiedBeforeRun = true
				record.BeforeRunSHA256 = observed.SHA256
			} else {
				record.VerifiedAfterRun = true
				record.AfterRunSHA256 = observed.SHA256
			}
			return nil
		}
	}
	return fmt.Errorf("generated executable %q has no retained producer evidence", relative)
}

func readProbeObservation(path string, plan ProbePlan) (ProbeObservation, []ProbeObservedChoice, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return ProbeObservation{}, nil, fmt.Errorf("read fresh probe observation: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ProbeObservation{}, nil, fmt.Errorf("fresh probe observation is not a regular non-symlink file")
	}
	if info.Size() > maxCapturedOutput {
		return ProbeObservation{}, nil, fmt.Errorf("fresh probe observation exceeds %d bytes", maxCapturedOutput)
	}
	file, err := os.Open(path)
	if err != nil {
		return ProbeObservation{}, nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, maxCapturedOutput+1))
	decoder.DisallowUnknownFields()
	var observation ProbeObservation
	if err := decoder.Decode(&observation); err != nil {
		return ProbeObservation{}, nil, fmt.Errorf("decode fresh probe observation: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ProbeObservation{}, nil, fmt.Errorf("fresh probe observation contains trailing JSON")
	}
	if len(observation.Traces) != len(plan.ExpectedSemantics.RuntimeOutcomes) {
		return ProbeObservation{}, nil, fmt.Errorf("fresh probe observation has %d traces, want %d", len(observation.Traces), len(plan.ExpectedSemantics.RuntimeOutcomes))
	}
	operationByID := make(map[string]semanticir.Operation, len(plan.Operations))
	for _, operation := range plan.Operations {
		operationByID[operation.ID] = operation
	}
	derived := make([]ProbeObservedChoice, 0, len(observation.Traces))
	for index, trace := range observation.Traces {
		runtime := plan.ExpectedSemantics.RuntimeOutcomes[index]
		operation, exists := operationByID[runtime.Behavior.OperationID]
		classified, classifyErr := semanticir.ClassifyRawOutcome(operation, trace, runtime.Behavior.Provenance)
		if !exists || classifyErr != nil {
			return ProbeObservation{}, nil, fmt.Errorf("classify raw probe trace %d: %v", index, classifyErr)
		}
		canonical, canonicalErr := semanticir.CanonicalJSON(trace)
		if canonicalErr != nil {
			return ProbeObservation{}, nil, fmt.Errorf("canonicalize raw probe trace %d: %v", index, canonicalErr)
		}
		decoded, decodeErr := decodeRawOutcomeSignal(canonical)
		if decodeErr != nil || !reflect.DeepEqual(decoded, trace) {
			return ProbeObservation{}, nil, fmt.Errorf("raw probe trace %d is not a closed canonical runtime record", index)
		}
		derived = append(derived, ProbeObservedChoice{Behavior: runtime.Behavior, RawOutcome: trace, ClassifiedOutcomeID: classified})
	}
	return observation, derived, nil
}

func probeObservationMatchesPlan(observation ProbeObservation, derived []ProbeObservedChoice, plan ProbePlan) bool {
	if len(observation.Traces) != len(plan.ExpectedSemantics.RuntimeOutcomes) || len(derived) != len(plan.ExpectedSemantics.RuntimeOutcomes) {
		return false
	}
	for index, runtime := range plan.ExpectedSemantics.RuntimeOutcomes {
		if !reflect.DeepEqual(observation.Traces[index], runtime.RawOutcome) || !reflect.DeepEqual(derived[index].Behavior, runtime.Behavior) || !reflect.DeepEqual(derived[index].RawOutcome, runtime.RawOutcome) || derived[index].ClassifiedOutcomeID != runtime.MappingOutcomeID {
			return false
		}
	}
	return true
}

// ValidateProbeConfirmation verifies the internal consistency of a confirmed
// direct-probe record without consulting mutable external state. Certificate
// issuance should call it in addition to validating the enclosing proof and
// frozen task manifest.
func ValidateProbeConfirmation(confirmation Confirmation) error {
	if confirmation.Mode != ConfirmationModeProbe || confirmation.Status != StatusConfirmed {
		return fmt.Errorf("probe confirmation is not confirmed probe-mode evidence")
	}
	if len(confirmation.PlanIDs) != 1 || len(confirmation.Plans) != 0 || confirmation.WitnessID == "" || len(confirmation.Materializations) != 0 || confirmation.ObservedTestPasses != nil {
		return fmt.Errorf("probe confirmation mixes edit evidence or has incomplete identity")
	}
	if len(confirmation.Blockers) != 0 || confirmation.Probe == nil {
		return fmt.Errorf("probe confirmation has blockers or no probe record")
	}
	evidence := confirmation.Probe
	plan := evidence.Plan
	if plan.ID != confirmation.PlanIDs[0] || plan.WitnessID != confirmation.WitnessID || plan.Witness.ID != confirmation.WitnessID {
		return fmt.Errorf("probe confirmation IDs do not match its retained plan")
	}
	if plan.Obligation != semanticir.ObligationReferenceCorrectness || plan.Witness.Obligation != plan.Obligation {
		return fmt.Errorf("probe confirmation has the wrong proof obligation")
	}
	if err := validateProbePlanRecord(plan); err != nil {
		return err
	}
	if err := validateProbeSemantics(plan.ExpectedSemantics, plan.Witness, plan.Operations); err != nil {
		return fmt.Errorf("probe plan semantics: %w", err)
	}
	if !reflect.DeepEqual(evidence.Expected, plan.ExpectedSemantics) || evidence.Observed == nil || !probeObservationMatchesPlan(*evidence.Observed, evidence.Derived, plan) || !evidence.SemanticsMatch || !evidence.FreshObservation {
		return fmt.Errorf("probe expected and observed semantics do not match exactly")
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"plan", evidence.PlanSHA256, mustProbeDigest(plan)},
		{"witness", evidence.WitnessSHA256, mustProbeDigest(plan.Witness)},
		{"expected semantics", evidence.ExpectedSHA256, mustProbeDigest(plan.ExpectedSemantics)},
		{"observed semantics", evidence.ObservedSHA256, mustProbeDigest(*evidence.Observed)},
		{"derived semantics", evidence.DerivedSHA256, mustProbeDigest(evidence.Derived)},
		{"workspace", evidence.WorkspaceSHA256, plan.Workspace.TreeSHA256},
		{"copied workspace", evidence.CopiedWorkspaceSHA256, plan.Workspace.TreeSHA256},
		{"harness", evidence.HarnessSHA256, plan.Harness.SHA256},
		{"harness bytes", digestBytes(plan.Harness.Bytes), plan.Harness.SHA256},
		{"staged harness", evidence.StagedHarnessSHA256, plan.Harness.SHA256},
		{"steps", evidence.StepsSHA256, mustProbeDigest(plan.Steps)},
		{"final command", evidence.CommandSHA256, mustProbeDigest(plan.Steps[len(plan.Steps)-1])},
	}
	for _, check := range checks {
		if !validDigest(check.got) || check.got != check.want {
			return fmt.Errorf("probe %s digest is inconsistent", check.name)
		}
	}
	if evidence.HarnessPath != plan.Harness.Path || evidence.ObservationPath != plan.Steps[len(plan.Steps)-1].ObservationPath || evidence.Error != "" {
		return fmt.Errorf("probe paths or error record are inconsistent")
	}
	if evidence.HarnessPreviouslyExisted {
		if !evidence.HarnessRestored || evidence.HarnessRemoved {
			return fmt.Errorf("pre-existing probe harness was not restored exactly")
		}
	} else if evidence.HarnessRestored || !evidence.HarnessRemoved {
		return fmt.Errorf("generated probe harness was not removed")
	}
	if evidence.IsolatedWorkspaceRoot == "" || !filepath.IsAbs(evidence.IsolatedWorkspaceRoot) || !evidence.IsolatedWorkspaceRemoved || !evidence.OriginalWorkspaceIntact || !evidence.ToolsVerifiedAfterRun {
		return fmt.Errorf("probe workspace cleanup is incomplete")
	}
	if err := validateProbeBindings(plan, evidence); err != nil {
		return err
	}
	if err := validateProbeStepEvidence(plan, evidence); err != nil {
		return err
	}
	if !reflect.DeepEqual(confirmation.Command, evidence.Steps[len(evidence.Steps)-1].Command) {
		return fmt.Errorf("probe final command differs from the retained final step")
	}
	return nil
}

func validateProbePlanRecord(plan ProbePlan) error {
	if plan.ID == "" || plan.WitnessID == "" || plan.Witness.ID != plan.WitnessID || plan.Obligation != semanticir.ObligationReferenceCorrectness {
		return fmt.Errorf("probe plan has incomplete witness identity")
	}
	if plan.Workspace.ID == "" || plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || !validDigest(plan.Workspace.TreeSHA256) || !filepath.IsAbs(plan.Workspace.Root) || filepath.Clean(plan.Workspace.Root) != plan.Workspace.Root {
		return fmt.Errorf("probe plan has an invalid workspace binding")
	}
	if len(plan.SourceArtifacts) == 0 || len(plan.Tools) == 0 {
		return fmt.Errorf("probe plan omits source or tool bindings")
	}
	witnessSourceBound := false
	for _, source := range plan.SourceArtifacts {
		if source.ID == "" || source.Path == "" || !validDigest(source.Digest) {
			return fmt.Errorf("probe plan has an invalid source binding")
		}
		if source.ID == plan.Witness.Provenance.ArtifactID && source.Digest == plan.Witness.Provenance.ArtifactDigest {
			witnessSourceBound = true
		}
	}
	if !witnessSourceBound {
		return fmt.Errorf("probe plan does not bind witness provenance to frozen source")
	}
	for _, tool := range plan.Tools {
		if tool.Name == "" || !filepath.IsAbs(tool.Path) || tool.Version == "" || !validDigest(tool.Digest) {
			return fmt.Errorf("probe plan has an invalid tool binding")
		}
	}
	if len(plan.Harness.Bytes) == 0 || plan.Harness.Mode == 0 || plan.Harness.Mode&^uint32(0o777) != 0 || digestBytes(plan.Harness.Bytes) != plan.Harness.SHA256 {
		return fmt.Errorf("probe plan has an invalid harness binding")
	}
	harness, err := cleanProbeRelative(plan.Harness.Path)
	if err != nil || filepath.ToSlash(harness) != plan.Harness.Path {
		return fmt.Errorf("probe plan has a non-canonical harness path")
	}
	if err := validateProbeStepRecords(plan.Steps, plan.Tools, harness); err != nil {
		return err
	}
	return nil
}

func validateProbeStepRecords(steps []ProbeStep, tools []semanticir.ToolRef, harness string) error {
	if len(steps) == 0 {
		return fmt.Errorf("probe plan has no ordered steps")
	}
	ids, outputs, verdicts := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for index, step := range steps {
		final := index == len(steps)-1
		if step.ID == "" || ids[step.ID] || final && step.Kind != ProbeStepRun || !final && step.Kind != ProbeStepCompile {
			return fmt.Errorf("probe plan has invalid step identity or order")
		}
		ids[step.ID] = true
		if len(step.Argv) == 0 || step.Timeout <= 0 || (step.PassSignal.ExitCode == nil) == (step.PassSignal.VerdictFile == nil) {
			return fmt.Errorf("probe step %q has incomplete invocation policy", step.ID)
		}
		for _, argument := range step.Argv {
			if strings.ContainsRune(argument, '\x00') || !utf8.ValidString(argument) {
				return fmt.Errorf("probe step %q argv contains non-canonical text", step.ID)
			}
		}
		work, err := cleanProbeRelativeAllowDot(step.WorkDir)
		if err != nil || step.WorkDir != filepath.ToSlash(work) {
			return fmt.Errorf("probe step %q has a non-canonical workdir", step.ID)
		}
		environmentNames := map[string]bool{}
		for _, entry := range step.Environment {
			name, _, ok := strings.Cut(entry, "=")
			if !ok || name == "" || strings.ContainsRune(entry, '\x00') || !utf8.ValidString(entry) || environmentNames[name] {
				return fmt.Errorf("probe step %q has an invalid exact environment", step.ID)
			}
			environmentNames[name] = true
		}
		toolMode, generatedMode := step.Tool != nil, step.GeneratedExecutable != ""
		if toolMode == generatedMode {
			return fmt.Errorf("probe step %q does not select exactly one executable kind", step.ID)
		}
		if toolMode {
			if step.Argv[0] != step.Tool.Path || !filepath.IsAbs(step.Tool.Path) {
				return fmt.Errorf("probe step %q argv[0] differs from its frozen tool", step.ID)
			}
			bound := false
			for _, tool := range tools {
				if reflect.DeepEqual(tool, *step.Tool) {
					bound = true
					break
				}
			}
			if !bound {
				return fmt.Errorf("probe step %q has an unbound tool", step.ID)
			}
		} else {
			executable, err := cleanProbeRelative(step.GeneratedExecutable)
			if step.Kind != ProbeStepRun || err != nil || step.GeneratedExecutable != filepath.ToSlash(executable) || step.Argv[0] != step.GeneratedExecutable || !outputs[executable] {
				return fmt.Errorf("probe step %q has an invalid generated executable", step.ID)
			}
		}
		if step.Kind == ProbeStepCompile {
			if !toolMode || len(step.Outputs) == 0 || step.ObservationPath != "" {
				return fmt.Errorf("compile step %q has an invalid shape", step.ID)
			}
			for _, raw := range step.Outputs {
				output, err := cleanProbeRelative(raw)
				if err != nil || raw != filepath.ToSlash(output) || outputs[output] || output == harness {
					return fmt.Errorf("compile step %q has invalid output %q", step.ID, raw)
				}
				outputs[output] = true
			}
		} else if len(step.Outputs) != 0 {
			return fmt.Errorf("run step %q declares outputs", step.ID)
		}
		if verdict := step.PassSignal.VerdictFile; verdict != nil {
			path, err := cleanProbeRelative(verdict.Path)
			if err != nil || verdict.Path != filepath.ToSlash(path) || strings.TrimSpace(verdict.PassValue) == "" || !utf8.ValidString(verdict.PassValue) || path == harness {
				return fmt.Errorf("probe step %q has an invalid verdict signal", step.ID)
			}
			verdicts[path] = true
		}
	}
	observation, err := cleanProbeRelative(steps[len(steps)-1].ObservationPath)
	if err != nil || steps[len(steps)-1].ObservationPath != filepath.ToSlash(observation) || observation == harness || outputs[observation] {
		return fmt.Errorf("probe plan has an invalid final observation path")
	}
	for output := range outputs {
		if verdicts[output] {
			return fmt.Errorf("probe generated output and verdict paths overlap")
		}
	}
	return nil
}

func validateProbeStepEvidence(plan ProbePlan, evidence *ProbeEvidence) error {
	if len(evidence.Steps) != len(plan.Steps) {
		return fmt.Errorf("probe step evidence cardinality differs from its plan")
	}
	outputs := make(map[string]*ProbeGeneratedOutputEvidence)
	for index, record := range evidence.Steps {
		step := plan.Steps[index]
		if !reflect.DeepEqual(record.Step, step) || record.StepSHA256 != mustProbeDigest(step) {
			return fmt.Errorf("probe step %d does not retain its exact plan and digest", index)
		}
		if err := validateProbeCommandEvidence(record.Command); err != nil {
			return fmt.Errorf("probe step %q: %w", step.ID, err)
		}
		wantArgv := append([]string(nil), step.Argv...)
		if step.GeneratedExecutable != "" {
			wantArgv[0] = filepath.Join(evidence.IsolatedWorkspaceRoot, filepath.FromSlash(step.GeneratedExecutable))
		}
		wantWorkDir := filepath.Join(evidence.IsolatedWorkspaceRoot, filepath.FromSlash(step.WorkDir))
		if !reflect.DeepEqual(record.Command.Command, wantArgv) || record.Command.WorkDir != wantWorkDir || record.Command.Timeout != step.Timeout || record.Command.EnvironmentSHA256 != digestEnvironment(step.Environment) {
			return fmt.Errorf("probe step %q command differs from exact argv/workdir/environment/timeout", step.ID)
		}
		if step.PassSignal.ExitCode != nil {
			if record.Command.Signal.Kind != "exit-code" || record.Command.Signal.ExpectedExitCode == nil || *record.Command.Signal.ExpectedExitCode != *step.PassSignal.ExitCode {
				return fmt.Errorf("probe step %q exit signal differs from its plan", step.ID)
			}
		} else {
			verdict := step.PassSignal.VerdictFile
			if verdict == nil || record.Command.Signal.Kind != "verdict-file" || record.Command.Signal.ExpectedValueSHA256 != digestBytes([]byte(strings.TrimSpace(verdict.PassValue))) || record.Command.Signal.VerdictPath != filepath.Join(evidence.IsolatedWorkspaceRoot, filepath.FromSlash(verdict.Path)) {
				return fmt.Errorf("probe step %q verdict signal differs from its plan", step.ID)
			}
		}
		if len(record.Outputs) != len(step.Outputs) {
			return fmt.Errorf("probe step %q output evidence cardinality differs", step.ID)
		}
		for outputIndex := range record.Outputs {
			output := &record.Outputs[outputIndex]
			if output.Path != step.Outputs[outputIndex] || output.ProducerStepID != step.ID || !output.Fresh || !validDigest(output.SHA256) || output.Size < 0 || output.Mode&^uint32(0o777) != 0 || outputs[output.Path] != nil {
				return fmt.Errorf("probe step %q has invalid generated output evidence", step.ID)
			}
			outputs[output.Path] = output
		}
	}
	for _, step := range plan.Steps {
		if step.GeneratedExecutable == "" {
			continue
		}
		output := outputs[step.GeneratedExecutable]
		if output == nil || output.Mode&0o111 == 0 || !output.VerifiedBeforeRun || !output.VerifiedAfterRun || output.BeforeRunSHA256 != output.SHA256 || output.AfterRunSHA256 != output.SHA256 {
			return fmt.Errorf("generated executable %q lacks immutable pre/post run evidence", step.GeneratedExecutable)
		}
	}
	return nil
}

func validateProbeBindings(plan ProbePlan, evidence *ProbeConfirmation) error {
	if len(evidence.SourceBindings) != len(plan.SourceArtifacts) || len(evidence.ToolBindings) != len(plan.Tools) {
		return fmt.Errorf("probe binding evidence cardinality does not match its plan")
	}
	for index, binding := range evidence.SourceBindings {
		ref := plan.SourceArtifacts[index]
		if !binding.Verified || binding.ID != ref.ID || binding.ExpectedSHA256 != ref.Digest || binding.ObservedSHA256 != ref.Digest || !validDigest(binding.ObservedSHA256) {
			return fmt.Errorf("probe source binding %d is not verified", index)
		}
	}
	for index, binding := range evidence.ToolBindings {
		tool := plan.Tools[index]
		if !binding.Verified || binding.ID != tool.Name || binding.ExpectedSHA256 != tool.Digest || binding.ObservedSHA256 != tool.Digest || binding.Version != tool.Version || !validDigest(binding.ObservedSHA256) {
			return fmt.Errorf("probe tool binding %d is not verified", index)
		}
	}
	return nil
}

func validateProbeCommandEvidence(command CommandEvidence) error {
	if !command.Passed || command.Error != "" || command.TimedOut || command.Interrupted || command.OutputTruncated || command.StartedAt.IsZero() {
		return fmt.Errorf("probe command lacks a complete passing execution record")
	}
	if !utf8.ValidString(command.Stdout) || !utf8.ValidString(command.Stderr) {
		return fmt.Errorf("probe command output is not canonical UTF-8 evidence")
	}
	if command.StdoutSHA256 != digestBytes([]byte(command.Stdout)) || command.StderrSHA256 != digestBytes([]byte(command.Stderr)) || !validDigest(command.CommandSHA256) || !validDigest(command.EnvironmentSHA256) {
		return fmt.Errorf("probe command output or invocation digests are inconsistent")
	}
	switch command.Signal.Kind {
	case "exit-code":
		if command.Signal.ExpectedExitCode == nil || command.Signal.ObservedExitCode == nil || *command.Signal.ExpectedExitCode != *command.Signal.ObservedExitCode {
			return fmt.Errorf("probe command has no matching fresh exit signal")
		}
		if command.SignalValueSHA256 != digestBytes([]byte(fmt.Sprint(*command.Signal.ObservedExitCode))) {
			return fmt.Errorf("probe exit signal digest is inconsistent")
		}
	case "verdict-file":
		if !command.Signal.FreshVerdict || command.Signal.ExpectedValueSHA256 != command.Signal.ObservedValueSHA256 || command.SignalValueSHA256 != command.Signal.ObservedValueSHA256 || !validDigest(command.SignalValueSHA256) {
			return fmt.Errorf("probe command has no matching fresh verdict-file signal")
		}
	default:
		return fmt.Errorf("probe command has unsupported pass-signal evidence")
	}
	return nil
}

func mustProbeDigest(value any) string {
	digest, err := semanticir.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}

func verifyPreparedProbeTools(tools []preparedBinding) error {
	for _, tool := range tools {
		resolved, err := filepath.EvalSymlinks(tool.tool.Path)
		if err != nil || filepath.Clean(resolved) != tool.path {
			return fmt.Errorf("tool %q path changed", tool.tool.Name)
		}
		info, err := os.Lstat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("tool %q is no longer a regular executable", tool.tool.Name)
		}
		body, err := os.ReadFile(resolved)
		if err != nil || digestBytes(body) != tool.digest {
			return fmt.Errorf("tool %q bytes changed", tool.tool.Name)
		}
	}
	return nil
}

// WorkspaceDigest calculates the canonical workspace-tree digest used by
// frozen task manifests. It hashes sorted relative paths, entry kinds, modes,
// sizes, file contents, and symlink targets without following symlinks.
func WorkspaceDigest(root string) (string, error) {
	entries, err := snapshotProbeWorkspace(root)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return digestBytes(encoded), nil
}

func snapshotProbeWorkspace(root string) ([]probeWorkspaceEntry, error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("workspace %q is not a directory", root)
	}
	var entries []probeWorkspaceEntry
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		record := probeWorkspaceEntry{Path: filepath.ToSlash(relative), Mode: uint32(info.Mode().Perm())}
		switch {
		case info.Mode().IsRegular():
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			record.Kind, record.Size, record.SHA256 = "file", int64(len(body)), digestBytes(body)
		case info.Mode()&os.ModeSymlink != 0:
			target, readErr := os.Readlink(path)
			if readErr != nil {
				return readErr
			}
			record.Kind, record.Size, record.SHA256 = "symlink", int64(len(target)), digestBytes([]byte(target))
		default:
			return fmt.Errorf("workspace entry %q has unsupported mode %s", relative, info.Mode())
		}
		entries = append(entries, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("workspace has no files")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func makeProbeWorkspaceCopy(sourceRoot, expectedDigest string) (string, string, string, error) {
	canonicalSource, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve source workspace: %w", err)
	}
	canonicalSource = filepath.Clean(canonicalSource)
	current, err := WorkspaceDigest(canonicalSource)
	if err != nil {
		return "", "", "", err
	}
	if current != expectedDigest {
		return "", "", "", fmt.Errorf("workspace became stale before isolated copy: got %s, want %s", current, expectedDigest)
	}
	tempParent, err := os.MkdirTemp("", "hyperray-probe-workspace-*")
	if err != nil {
		return "", "", "", err
	}
	canonicalParent, err := filepath.EvalSymlinks(tempParent)
	if err != nil {
		os.RemoveAll(tempParent)
		return "", "", "", fmt.Errorf("resolve isolated workspace parent: %w", err)
	}
	tempParent = filepath.Clean(canonicalParent)
	runRoot := filepath.Join(tempParent, "workspace")
	if err := copyProbeTree(canonicalSource, runRoot); err != nil {
		os.RemoveAll(tempParent)
		return "", "", "", err
	}
	copied, err := WorkspaceDigest(runRoot)
	if err != nil || copied != expectedDigest {
		os.RemoveAll(tempParent)
		if err == nil {
			err = fmt.Errorf("isolated workspace digest is %s, want %s", copied, expectedDigest)
		}
		return "", "", "", err
	}
	return tempParent, runRoot, copied, nil
}

func copyProbeTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported workspace entry %q", relative)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, body, info.Mode().Perm()); err != nil {
			return err
		}
		return nil
	})
}

func cleanProbeRelative(path string) (string, error) {
	clean, err := cleanProbeRelativeAllowDot(path)
	if err != nil {
		return "", err
	}
	if clean == "." {
		return "", fmt.Errorf("path must name a file")
	}
	return clean, nil
}

func cleanProbeRelativeAllowDot(path string) (string, error) {
	if !utf8.ValidString(path) {
		return "", fmt.Errorf("path contains non-canonical text")
	}
	if path == "" {
		path = "."
	}
	path = filepath.FromSlash(path)
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("path %q must be workspace-relative", path)
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes the workspace", path)
	}
	return clean, nil
}

func resolveProbeExisting(root, declared string, directory bool) (string, error) {
	relative, err := cleanProbeRelativeAllowDot(declared)
	if err != nil {
		return "", err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	canonicalRoot = filepath.Clean(canonicalRoot)
	candidate := filepath.Join(canonicalRoot, relative)
	leaf, err := os.Lstat(candidate)
	if err != nil {
		return "", err
	}
	if leaf.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("path %q is a leaf symlink", declared)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil || !pathWithin(canonicalRoot, filepath.Clean(resolved)) {
		return "", fmt.Errorf("path %q resolves outside the workspace", declared)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if directory && !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", declared)
	}
	if !directory && !info.Mode().IsRegular() {
		return "", fmt.Errorf("path %q is not a regular file", declared)
	}
	return filepath.Clean(resolved), nil
}

func resolveProbeParent(root, declared string) (string, error) {
	relative, err := cleanProbeRelative(declared)
	if err != nil {
		return "", err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	canonicalRoot = filepath.Clean(canonicalRoot)
	parent := filepath.Join(canonicalRoot, filepath.Dir(relative))
	existing := parent
	var missing []string
	for {
		info, inspectErr := os.Lstat(existing)
		if inspectErr == nil {
			if !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
				return "", fmt.Errorf("path %q parent is not a directory", declared)
			}
			break
		}
		if !os.IsNotExist(inspectErr) || existing == canonicalRoot {
			return "", fmt.Errorf("path %q parent is not a directory", declared)
		}
		missing = append(missing, filepath.Base(existing))
		existing = filepath.Dir(existing)
	}
	resolvedParent, err := filepath.EvalSymlinks(existing)
	if err != nil || !pathWithin(canonicalRoot, filepath.Clean(resolvedParent)) {
		return "", fmt.Errorf("path %q has a parent outside the workspace", declared)
	}
	resolvedInfo, err := os.Stat(resolvedParent)
	if err != nil || !resolvedInfo.IsDir() {
		return "", fmt.Errorf("path %q parent is not a directory", declared)
	}
	for index := len(missing) - 1; index >= 0; index-- {
		resolvedParent = filepath.Join(resolvedParent, missing[index])
	}
	if !pathWithin(canonicalRoot, resolvedParent) {
		return "", fmt.Errorf("path %q has a parent outside the workspace", declared)
	}
	target := filepath.Join(resolvedParent, filepath.Base(relative))
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("path %q target has an unsupported type", declared)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return target, nil
}

func snapshotLocalFile(path string) (localFileSnapshot, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return localFileSnapshot{}, nil
	}
	if err != nil {
		return localFileSnapshot{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return localFileSnapshot{}, fmt.Errorf("path %q is not a regular non-symlink file", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return localFileSnapshot{}, err
	}
	return localFileSnapshot{existed: true, body: body, mode: info.Mode().Perm()}, nil
}

func restoreLocalFile(path string, snapshot localFileSnapshot) error {
	if snapshot.existed {
		return writeExact(path, snapshot.body, snapshot.mode)
	}
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func localFileEquals(path string, expected []byte) bool {
	body, err := os.ReadFile(path)
	return err == nil && bytes.Equal(body, expected)
}
