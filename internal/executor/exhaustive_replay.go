package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// ExhaustiveReplayPlan binds one frontend-produced exhaustive execution
// record to the frozen workspace and source bytes from which it was derived.
// The executor never derives assignments or semantic outcomes from execution;
// it only replays and compares the already-declared raw observations.
type ExhaustiveReplayPlan struct {
	ID              string                                 `json:"id"`
	Workspace       ProbeWorkspace                         `json:"workspace"`
	SourceArtifacts []semanticir.ArtifactRef               `json:"source_artifacts"`
	Operations      []semanticir.Operation                 `json:"operations"`
	Evidence        semanticir.ExhaustiveExecutionEvidence `json:"evidence"`
}

// ExhaustiveReplayCommandEvidence records the exact bytes observed from one
// shell-free setup, run, or cleanup process.
type ExhaustiveReplayCommandEvidence struct {
	Step              semanticir.ProbeStep `json:"step"`
	StepSHA256        string               `json:"step_sha256"`
	ExecutablePath    string               `json:"executable_path"`
	ExecutableSHA256  string               `json:"executable_sha256"`
	Argv              []string             `json:"argv"`
	WorkingDirectory  string               `json:"working_directory"`
	EnvironmentSHA256 string               `json:"environment_sha256"`
	StdinSHA256       string               `json:"stdin_sha256"`
	StartedAt         time.Time            `json:"started_at"`
	Duration          time.Duration        `json:"duration"`
	ExitCode          *int                 `json:"exit_code,omitempty"`
	Stdout            []byte               `json:"stdout"`
	StdoutSHA256      string               `json:"stdout_sha256"`
	Stderr            []byte               `json:"stderr"`
	StderrSHA256      string               `json:"stderr_sha256"`
	SignalValue       []byte               `json:"signal_value"`
	SignalSHA256      string               `json:"signal_sha256"`
	SignalPath        string               `json:"signal_path,omitempty"`
	FreshSignal       bool                 `json:"fresh_signal"`
	SignalRemoved     bool                 `json:"signal_removed"`
	OutputTruncated   bool                 `json:"output_truncated"`
	TimedOut          bool                 `json:"timed_out"`
	Interrupted       bool                 `json:"interrupted"`
	Passed            bool                 `json:"passed"`
	Error             string               `json:"error,omitempty"`
}

// ExhaustiveGeneratedOutputEvidence proves a setup output was fresh, regular,
// non-symlinked, byte-bound, executable when declared, unchanged across its
// consumer run, and removed before the disposable copy was deleted.
type ExhaustiveGeneratedOutputEvidence struct {
	Output            semanticir.ProbeOutput `json:"output"`
	ProducerStepID    string                 `json:"producer_step_id"`
	Path              string                 `json:"path"`
	Fresh             bool                   `json:"fresh"`
	SHA256            string                 `json:"sha256"`
	Mode              uint32                 `json:"mode"`
	Size              int64                  `json:"size"`
	VerifiedBeforeRun bool                   `json:"verified_before_run"`
	BeforeRunSHA256   string                 `json:"before_run_sha256"`
	VerifiedAfterRun  bool                   `json:"verified_after_run"`
	AfterRunSHA256    string                 `json:"after_run_sha256"`
	RemovedAfterRun   bool                   `json:"removed_after_run"`
	RemovalError      string                 `json:"removal_error,omitempty"`
}

// ExhaustiveHarnessEvidence records exact staging and restoration/removal of
// the frontend-generated harness inside one disposable workspace.
type ExhaustiveHarnessEvidence struct {
	DeclaredPath      string `json:"declared_path"`
	StagedPath        string `json:"staged_path"`
	ExpectedSHA256    string `json:"expected_sha256"`
	StagedSHA256      string `json:"staged_sha256"`
	PreviouslyExisted bool   `json:"previously_existed"`
	PreviousSHA256    string `json:"previous_sha256,omitempty"`
	PreviousMode      uint32 `json:"previous_mode,omitempty"`
	Restored          bool   `json:"restored"`
	Removed           bool   `json:"removed"`
	RestorationError  string `json:"restoration_error,omitempty"`
}

// ExhaustiveObservationReplay is one assignment execution in its own fresh
// copy. Setup is rerun in that copy; no compiler or harness state is shared
// with any other assignment or repetition order.
type ExhaustiveObservationReplay struct {
	RunID                 string                              `json:"run_id"`
	ObservationIndex      int                                 `json:"observation_index"`
	Expected              semanticir.ExecutionObservation     `json:"expected"`
	ExpectedSHA256        string                              `json:"expected_sha256"`
	Setup                 []ExhaustiveReplayCommandEvidence   `json:"setup"`
	Run                   ExhaustiveReplayCommandEvidence     `json:"run"`
	Cleanup               []ExhaustiveReplayCommandEvidence   `json:"cleanup"`
	GeneratedOutputs      []ExhaustiveGeneratedOutputEvidence `json:"generated_outputs"`
	Harness               ExhaustiveHarnessEvidence           `json:"harness"`
	Isolation             IsolationEvidence                   `json:"isolation"`
	OutputsMatch          bool                                `json:"outputs_match"`
	FreshProcess          bool                                `json:"fresh_process"`
	ToolsVerifiedAfterRun bool                                `json:"tools_verified_after_run"`
	Error                 string                              `json:"error,omitempty"`
}

// ExhaustiveRunReplay retains one declared order and every independently
// replayed observation in that order.
type ExhaustiveRunReplay struct {
	Run               semanticir.ExecutionRunEvidence `json:"run"`
	RunSHA256         string                          `json:"run_sha256"`
	OrderDigest       string                          `json:"order_digest"`
	ObservationDigest string                          `json:"observation_digest"`
	Observations      []ExhaustiveObservationReplay   `json:"observations"`
	Complete          bool                            `json:"complete"`
}

// ExhaustiveReplayEvidence is the certificate-facing result. CONFIRMED means
// all raw observations were reproduced from actual process output in two
// declared orders, with one disposable copy and process per assignment.
type ExhaustiveReplayEvidence struct {
	Plan                    ExhaustiveReplayPlan                `json:"plan"`
	PlanSHA256              string                              `json:"plan_sha256"`
	ExecutionSHA256         string                              `json:"execution_sha256"`
	WorkspaceSHA256         string                              `json:"workspace_sha256"`
	SourceBindings          []BindingEvidence                   `json:"source_bindings"`
	ToolBindings            []BindingEvidence                   `json:"tool_bindings"`
	Runs                    []ExhaustiveRunReplay               `json:"runs"`
	SemanticReplay          semanticir.ExhaustiveReplayEvidence `json:"semantic_replay"`
	SemanticReplaySHA256    string                              `json:"semantic_replay_sha256"`
	Status                  Status                              `json:"status"`
	OriginalBeforeSHA256    string                              `json:"original_before_sha256"`
	OriginalAfterSHA256     string                              `json:"original_after_sha256"`
	OriginalWorkspaceIntact bool                                `json:"original_workspace_intact"`
	Blockers                []Blocker                           `json:"blockers,omitempty"`
}

type preparedExhaustiveReplay struct {
	plan    ExhaustiveReplayPlan
	root    string
	harness string
	sources []preparedBinding
	tools   []preparedBinding
	setup   []semanticir.ProbeStep
	runs    map[string]semanticir.ProbeStep
	cleanup []semanticir.ProbeStep
	outputs map[string]preparedReplayOutput
}

type preparedReplayOutput struct {
	output     semanticir.ProbeOutput
	producerID string
	workRel    string
	pathRel    string
}

// ReplayExhaustive independently re-executes a frontend's exhaustive raw
// observation transcript. It is deliberately fail-closed: incompatible
// normalized transcripts, deleted temporary paths, unbound tools, and any
// output that cannot be derived from process bytes are blockers.
func ReplayExhaustive(ctx context.Context, plan ExhaustiveReplayPlan) (result ExhaustiveReplayEvidence) {
	result.Plan = plan
	result.Status = StatusProofBlocked
	result.PlanSHA256, _ = semanticir.Digest(plan)
	result.ExecutionSHA256, _ = semanticir.Digest(plan.Evidence)
	result.WorkspaceSHA256 = plan.Workspace.TreeSHA256
	if ctx == nil {
		result.Blockers = append(result.Blockers, replayBlocker(plan.ID, "nil-context", "execution context is nil"))
		return result
	}
	prepared, blocker := prepareExhaustiveReplay(plan)
	if blocker != nil {
		result.Blockers = append(result.Blockers, *blocker)
		return result
	}
	result.OriginalBeforeSHA256 = plan.Workspace.TreeSHA256
	for _, source := range prepared.sources {
		result.SourceBindings = append(result.SourceBindings, BindingEvidence{
			ID: source.ref.ID, Path: source.path, ExpectedSHA256: source.ref.Digest,
			ObservedSHA256: source.digest, Verified: source.ref.Digest == source.digest,
		})
	}
	for _, tool := range prepared.tools {
		result.ToolBindings = append(result.ToolBindings, BindingEvidence{
			ID: tool.tool.Name, Path: tool.path, ExpectedSHA256: tool.tool.Digest,
			ObservedSHA256: tool.digest, Version: tool.version, Verified: tool.tool.Digest == tool.digest,
		})
	}

	defer func() {
		after, err := WorkspaceDigest(prepared.root)
		if err != nil {
			result.Blockers = append(result.Blockers, replayBlocker(plan.ID, "original-workspace-unreadable", err.Error()))
		} else {
			result.OriginalAfterSHA256 = after
			result.OriginalWorkspaceIntact = after == plan.Workspace.TreeSHA256
			if !result.OriginalWorkspaceIntact {
				result.Blockers = append(result.Blockers, replayBlocker(plan.ID, "original-workspace-mutated", "frozen workspace changed during exhaustive replay"))
			}
		}
		if err := verifyPreparedProbeTools(prepared.tools); err != nil {
			result.Blockers = append(result.Blockers, replayBlocker(plan.ID, "replay-tool-mutated", err.Error()))
		}
		if len(result.Blockers) == 0 && result.OriginalWorkspaceIntact && len(result.Runs) == 2 {
			semanticReplay, replayErr := semanticReplayForPlan(plan)
			if replayErr != nil {
				result.Blockers = append(result.Blockers, replayBlocker(plan.ID, "semantic-replay-conversion", replayErr.Error()))
			} else {
				result.SemanticReplay = semanticReplay
				result.SemanticReplaySHA256, _ = semanticir.Digest(semanticReplay)
				result.Status = StatusConfirmed
			}
		}
	}()

	for _, declaredRun := range plan.Evidence.Runs {
		run := ExhaustiveRunReplay{Run: declaredRun, OrderDigest: declaredRun.OrderDigest, ObservationDigest: declaredRun.ObservationDigest}
		run.RunSHA256, _ = semanticir.Digest(declaredRun)
		for index, observation := range declaredRun.Observations {
			replayed := replayExhaustiveObservation(ctx, prepared, declaredRun.ID, index, observation)
			run.Observations = append(run.Observations, replayed)
			if replayed.Error != "" {
				result.Blockers = append(result.Blockers, replayBlocker(plan.ID, "exhaustive-observation-mismatch", fmt.Sprintf("run %q observation %d: %s", declaredRun.ID, index, replayed.Error)))
				result.Runs = append(result.Runs, run)
				return result
			}
		}
		run.Complete = len(run.Observations) == len(declaredRun.Observations)
		result.Runs = append(result.Runs, run)
	}
	return result
}

func replayBlocker(planID, code, detail string) Blocker {
	return Blocker{Stage: "exhaustive-replay", PlanID: planID, Code: code, Detail: detail}
}

func prepareExhaustiveReplay(plan ExhaustiveReplayPlan) (preparedExhaustiveReplay, *Blocker) {
	block := func(code, detail string) (preparedExhaustiveReplay, *Blocker) {
		return preparedExhaustiveReplay{}, func() *Blocker { value := replayBlocker(plan.ID, code, detail); return &value }()
	}
	if strings.TrimSpace(plan.ID) == "" || plan.Evidence.ID == "" || !plan.Evidence.Complete {
		return block("invalid-replay-plan", "plan and exhaustive evidence must have identities and evidence must be complete")
	}
	if plan.Workspace.ID == "" || plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || !validDigest(plan.Workspace.TreeSHA256) || !filepath.IsAbs(plan.Workspace.Root) || filepath.Clean(plan.Workspace.Root) != plan.Workspace.Root {
		return block("invalid-replay-workspace", "replay must bind a canonical solution+new-tests workspace")
	}
	root, err := filepath.EvalSymlinks(plan.Workspace.Root)
	if err != nil {
		return block("invalid-replay-workspace", err.Error())
	}
	root = filepath.Clean(root)
	if digest, digestErr := WorkspaceDigest(root); digestErr != nil || digest != plan.Workspace.TreeSHA256 || plan.Evidence.WorkspaceTreeDigest != plan.Workspace.TreeSHA256 {
		return block("stale-replay-workspace", fmt.Sprintf("workspace/evidence digest mismatch: observed=%s expected=%s error=%v", digest, plan.Workspace.TreeSHA256, digestErr))
	}
	if len(plan.SourceArtifacts) == 0 {
		return block("missing-replay-sources", "replay declares no frozen source artifacts")
	}
	sources := make([]preparedBinding, 0, len(plan.SourceArtifacts))
	seenSourceIDs, seenSourcePaths, sourceDigestBound := map[string]bool{}, map[string]bool{}, false
	for _, source := range plan.SourceArtifacts {
		if source.ID == "" || source.Path == "" || !validDigest(source.Digest) {
			return block("invalid-replay-source", "source binding is incomplete")
		}
		path, pathErr := resolveProbeExisting(root, source.Path, false)
		if pathErr != nil {
			return block("replay-source-path", pathErr.Error())
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil || digestBytes(body) != source.Digest {
			return block("stale-replay-source", fmt.Sprintf("source %q differs from its frozen digest", source.ID))
		}
		if seenSourceIDs[source.ID] || seenSourcePaths[path] {
			return block("duplicate-replay-source", "source IDs and resolved paths must be unique")
		}
		seenSourceIDs[source.ID], seenSourcePaths[path] = true, true
		sourceDigestBound = sourceDigestBound || source.Digest == plan.Evidence.SourceDigest
		sources = append(sources, preparedBinding{ref: source, path: path, digest: source.Digest})
	}
	if !sourceDigestBound || !validDigest(plan.Evidence.SourceDigest) {
		return block("unbound-replay-source", "exhaustive source digest is not bound by a frozen source artifact")
	}
	operations := make(map[string]semanticir.Operation, len(plan.Operations))
	for _, operation := range plan.Operations {
		if operation.ID == "" || operations[operation.ID].ID != "" || len(operation.OutcomeIDs) == 0 {
			return block("invalid-replay-operation-alphabet", "operation outcome alphabets must have unique identities and non-empty outcomes")
		}
		seenOutcomes := map[string]bool{}
		for _, outcomeID := range operation.OutcomeIDs {
			if outcomeID == "" || seenOutcomes[outcomeID] {
				return block("invalid-replay-operation-alphabet", fmt.Sprintf("operation %q has an empty or duplicate outcome ID", operation.ID))
			}
			seenOutcomes[outcomeID] = true
		}
		operations[operation.ID] = operation
	}
	if len(operations) == 0 {
		return block("missing-replay-operation-alphabet", "replay declares no frozen operation outcome alphabet")
	}
	if len(plan.Evidence.Harness) == 0 || digestBytes(plan.Evidence.Harness) != plan.Evidence.HarnessDigest {
		return block("invalid-replay-harness", "harness bytes differ from their declared digest")
	}
	harness, err := cleanProbeRelative(plan.Evidence.HarnessPath)
	if err != nil || plan.Evidence.HarnessPath != filepath.ToSlash(harness) {
		return block("invalid-replay-harness-path", "harness path must be canonical and workspace-relative")
	}
	if _, err := resolveProbeParent(root, harness); err != nil {
		return block("invalid-replay-harness-path", err.Error())
	}
	for _, source := range sources {
		if samePath(source.path, filepath.Join(root, harness)) {
			return block("replay-harness-source-collision", "generated harness path aliases frozen source")
		}
	}

	if len(plan.Evidence.Replay.CleanupSteps) != 0 {
		return block("invalid-replay-cleanup", "external cleanup commands are forbidden; cleanup is programmatic and path-bound")
	}
	steps := append([]semanticir.ProbeStep(nil), plan.Evidence.Steps...)
	if diagnostics := semanticir.ValidateProbeSteps(steps, plan.Evidence.Provenance); semanticir.HasErrors(diagnostics) {
		return block("invalid-replay-steps", diagnostics[0].Message)
	}
	if len(plan.Evidence.Runs) != 2 {
		return block("invalid-replay-runs", "exhaustive replay requires exactly two declared repetition orders")
	}
	toolsByKey := map[string]preparedBinding{}
	bindTool := func(tool semanticir.ToolRef) error {
		if tool == (semanticir.ToolRef{}) {
			return nil
		}
		key := tool.Name + "\x00" + tool.Path
		if prior, exists := toolsByKey[key]; exists {
			if !reflect.DeepEqual(prior.tool, tool) {
				return fmt.Errorf("tool %q has inconsistent frozen bindings", tool.Name)
			}
			return nil
		}
		if tool.Name == "" || tool.Version == "" || !filepath.IsAbs(tool.Path) || !validDigest(tool.Digest) {
			return fmt.Errorf("tool %q has incomplete frozen identity", tool.Name)
		}
		path, resolveErr := filepath.EvalSymlinks(tool.Path)
		if resolveErr != nil {
			return fmt.Errorf("resolve tool %q: %w", tool.Name, resolveErr)
		}
		info, statErr := os.Lstat(path)
		body, readErr := os.ReadFile(path)
		if statErr != nil || readErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || digestBytes(body) != tool.Digest {
			return fmt.Errorf("tool %q is not the declared regular executable", tool.Name)
		}
		toolsByKey[key] = preparedBinding{tool: tool, path: filepath.Clean(path), digest: tool.Digest, version: tool.Version}
		return nil
	}
	if err := bindTool(plan.Evidence.Tool); err != nil {
		return block("invalid-replay-tool", err.Error())
	}
	for _, step := range steps {
		if err := bindTool(step.Tool); err != nil {
			return block("invalid-replay-tool", err.Error())
		}
	}
	tools := make([]preparedBinding, 0, len(toolsByKey))
	for _, tool := range toolsByKey {
		tools = append(tools, tool)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].tool.Name+"\x00"+tools[i].tool.Path < tools[j].tool.Name+"\x00"+tools[j].tool.Path
	})

	prepared := preparedExhaustiveReplay{plan: plan, root: root, harness: harness, sources: sources, tools: tools, runs: map[string]semanticir.ProbeStep{}, outputs: map[string]preparedReplayOutput{}}
	seenSteps := map[string]bool{}
	outputPaths := map[string]bool{}
	runObserved := map[string]bool{}
	for _, run := range plan.Evidence.Runs {
		if run.ID == "" || len(run.Observations) == 0 || run.FreshProcessCount != len(run.Observations) || run.ObservationDigest != plan.Evidence.CompleteAssignmentDigest {
			return block("invalid-replay-runs", "run identity, process count, or complete-assignment digest is inconsistent")
		}
		observationDigest, observationErr := semanticir.ExecutionObservationDigest(run.Observations)
		orderDigest, orderErr := semanticir.ExecutionOrderDigest(run.Observations)
		if observationErr != nil || orderErr != nil || observationDigest != run.ObservationDigest || orderDigest != run.OrderDigest {
			return block("invalid-replay-runs", "run digests differ from embedded raw observations")
		}
		for _, observation := range run.Observations {
			runObserved[observation.StepID] = true
			if digestBytes(observation.Stdout) != observation.StdoutDigest || digestBytes(observation.Stderr) != observation.StderrDigest || digestBytes(observation.SignalValue) != observation.SignalValueDigest || observation.StdoutTruncated || observation.StderrTruncated || observation.SignalTruncated {
				return block("invalid-replay-observation", "raw observation bytes, digests, or truncation flags are inconsistent")
			}
			operation, operationExists := operations[observation.Behavior.OperationID]
			canonicalRaw, canonicalErr := semanticir.CanonicalJSON(observation.RawOutcome)
			classified, classifyErr := semanticir.ClassifyRawOutcome(operation, observation.RawOutcome, observation.Provenance)
			if !operationExists || canonicalErr != nil || classifyErr != nil || !bytes.Equal(canonicalRaw, observation.SignalValue) || len(observation.OutcomeIDs) != 1 || observation.OutcomeIDs[0] != classified {
				return block("invalid-replay-observation", "signal is not exact canonical JSON for the declared raw outcome")
			}
		}
	}
	if len(plan.Evidence.Runs[0].Observations) != len(plan.Evidence.Runs[1].Observations) || plan.Evidence.Runs[0].ObservationDigest != plan.Evidence.Runs[1].ObservationDigest {
		return block("invalid-replay-runs", "declared repetition orders do not cover the same complete observation relation")
	}
	if len(plan.Evidence.Runs[0].Observations) > 1 && plan.Evidence.Runs[0].OrderDigest == plan.Evidence.Runs[1].OrderDigest {
		return block("invalid-replay-runs", "multi-assignment repetitions do not declare independent orders")
	}
	for _, step := range plan.Evidence.Steps {
		if seenSteps[step.ID] {
			return block("invalid-replay-steps", "step IDs must be unique")
		}
		seenSteps[step.ID] = true
		if err := validateReplayStepRoot(root, step); err != nil {
			return block("invalid-replay-steps", err.Error())
		}
		switch step.Kind {
		case semanticir.ProbeStepSetup:
			prepared.setup = append(prepared.setup, step)
			for _, output := range step.Outputs {
				if output.ExistedBefore || output.BeforeDigest != "" {
					return block("nonfresh-replay-output", fmt.Sprintf("setup output %q must be fresh", output.ID))
				}
				workRel, _ := replayWorkRelative(root, step.WorkingDirectory)
				canonical, pathErr := replayOutputRelative(root, workRel, output.Path)
				if pathErr != nil || canonical == harness {
					return block("invalid-replay-output-path", fmt.Sprintf("output %q escapes or aliases the harness", output.ID))
				}
				if _, duplicate := prepared.outputs[output.ID]; duplicate {
					return block("duplicate-replay-output", fmt.Sprintf("output ID %q is duplicated", output.ID))
				}
				if outputPaths[canonical] {
					return block("duplicate-replay-output", fmt.Sprintf("output %q aliases another generated path", output.ID))
				}
				prepared.outputs[output.ID] = preparedReplayOutput{output: output, producerID: step.ID, workRel: workRel, pathRel: canonical}
				outputPaths[canonical] = true
			}
		case semanticir.ProbeStepRun:
			prepared.runs[step.ID] = step
		default:
			return block("invalid-replay-steps", "primary evidence contains a cleanup step")
		}
	}
	executableBound := false
	for _, output := range prepared.outputs {
		if output.output.Executable && output.output.AfterDigest == plan.Evidence.ExecutableDigest {
			executableBound = true
		}
	}
	if !executableBound {
		for _, step := range prepared.runs {
			if step.GeneratedExecutableID == "" && step.Tool.Digest == plan.Evidence.ExecutableDigest {
				executableBound = true
			}
		}
	}
	if !executableBound || !validDigest(plan.Evidence.ExecutableDigest) {
		return block("unbound-replay-executable", "executable digest is not bound to a frozen tool or fresh generated output")
	}
	for stepID := range runObserved {
		if _, exists := prepared.runs[stepID]; !exists {
			return block("missing-replay-run-step", fmt.Sprintf("observation step %q is absent", stepID))
		}
	}
	for stepID := range prepared.runs {
		if !runObserved[stepID] {
			return block("unobserved-replay-run-step", fmt.Sprintf("run step %q has no raw observation", stepID))
		}
	}
	if len(prepared.runs) == 0 {
		return block("missing-replay-run-step", "exhaustive evidence has no run steps")
	}
	for _, run := range plan.Evidence.Runs {
		for _, observation := range run.Observations {
			step := prepared.runs[observation.StepID]
			if step.ExpectedExitCode != observation.ExitCode || step.ExpectedStdoutDigest != observation.StdoutDigest || step.ExpectedStderrDigest != observation.StderrDigest || step.ExpectedSignalDigest != observation.SignalValueDigest {
				return block("detached-replay-observation", fmt.Sprintf("observation %q differs from its run step declaration", observation.StepID))
			}
		}
	}
	return prepared, nil
}

func validateReplayStepRoot(root string, step semanticir.ProbeStep) error {
	if step.ID == "" || step.TimeoutMillis <= 0 || !step.ClearEnvironment || !step.KillProcessGroup || step.StdinDigest != digestBytes(step.Stdin) {
		return fmt.Errorf("step %q has incomplete hermetic execution policy", step.ID)
	}
	if _, err := replayWorkRelative(root, step.WorkingDirectory); err != nil {
		return fmt.Errorf("step %q working directory: %w", step.ID, err)
	}
	if digest, err := semanticir.Digest(step.Environment); err != nil || digest != step.EnvironmentDigest {
		return fmt.Errorf("step %q environment differs from its digest", step.ID)
	}
	names := map[string]bool{}
	for _, variable := range step.Environment {
		if variable.Name == "" || strings.ContainsRune(variable.Name, '=') || strings.ContainsRune(variable.Name, '\x00') || strings.ContainsRune(variable.Value, '\x00') || !utf8.ValidString(variable.Name) || !utf8.ValidString(variable.Value) || names[variable.Name] {
			return fmt.Errorf("step %q has an invalid exact environment", step.ID)
		}
		names[variable.Name] = true
	}
	for _, argument := range step.Argv {
		if argument == "" || strings.ContainsRune(argument, '\x00') || !utf8.ValidString(argument) {
			return fmt.Errorf("step %q has a non-canonical argument", step.ID)
		}
		if filepath.IsAbs(argument) && !pathWithin(root, filepath.Clean(argument)) {
			return fmt.Errorf("step %q argv contains an absolute path outside the frozen workspace", step.ID)
		}
	}
	return nil
}

func replayWorkRelative(root, declared string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	canonicalRoot = filepath.Clean(canonicalRoot)
	path := declared
	if !filepath.IsAbs(path) {
		path = filepath.Join(canonicalRoot, filepath.FromSlash(path))
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil || !pathWithin(canonicalRoot, filepath.Clean(path)) {
		return "", fmt.Errorf("path resolves outside frozen workspace")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}
	relative, err := filepath.Rel(canonicalRoot, path)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path cannot be represented inside frozen workspace")
	}
	return relative, nil
}

func replayOutputRelative(root, workRel, declared string) (string, error) {
	if declared == "" || filepath.IsAbs(declared) {
		return "", fmt.Errorf("generated output path must be relative")
	}
	target := filepath.Clean(filepath.Join(workRel, filepath.FromSlash(declared)))
	if target == ".." || strings.HasPrefix(target, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generated output escapes workspace")
	}
	if _, err := resolveProbeParent(root, target); err != nil {
		return "", err
	}
	return target, nil
}

func replayExhaustiveObservation(ctx context.Context, prepared preparedExhaustiveReplay, runID string, index int, observation semanticir.ExecutionObservation) (record ExhaustiveObservationReplay) {
	record.RunID, record.ObservationIndex, record.Expected = runID, index, observation
	record.ExpectedSHA256, _ = semanticir.Digest(observation)
	tempParent, runRoot, copiedDigest, err := makeProbeWorkspaceCopy(prepared.root, prepared.plan.Workspace.TreeSHA256)
	record.Isolation = IsolationEvidence{
		OriginalRoot: prepared.root, ExpectedSHA256: prepared.plan.Workspace.TreeSHA256,
		OriginalBeforeSHA256: prepared.plan.Workspace.TreeSHA256, CopyBeforeSHA256: copiedDigest,
	}
	if err != nil {
		record.Error = "create fresh observation workspace: " + err.Error()
		return record
	}
	record.Isolation.IsolatedRoot = runRoot
	originalTask := TaskEnvironment{WorkspaceRoot: prepared.root, WorkspaceSHA256: prepared.plan.Workspace.TreeSHA256}
	defer func() {
		if cleanupErr := finalizeIsolation(tempParent, runRoot, originalTask, &record.Isolation); cleanupErr != nil {
			record.Error = appendError(record.Error, cleanupErr.Error())
		}
		if !record.Isolation.IsolatedRemoved || !record.Isolation.OriginalIntact {
			record.Error = appendError(record.Error, "observation workspace cleanup or original-workspace preservation failed")
		}
	}()

	harnessPath := filepath.Join(runRoot, filepath.FromSlash(prepared.harness))
	if err := os.MkdirAll(filepath.Dir(harnessPath), 0o750); err != nil {
		record.Error = "create harness parent: " + err.Error()
		return record
	}
	harnessSnapshot, err := snapshotLocalFile(harnessPath)
	if err != nil {
		record.Error = "snapshot harness path: " + err.Error()
		return record
	}
	record.Harness = ExhaustiveHarnessEvidence{
		DeclaredPath: prepared.plan.Evidence.HarnessPath, StagedPath: prepared.harness,
		ExpectedSHA256: prepared.plan.Evidence.HarnessDigest, PreviouslyExisted: harnessSnapshot.existed,
	}
	if harnessSnapshot.existed {
		record.Harness.PreviousSHA256, record.Harness.PreviousMode = digestBytes(harnessSnapshot.body), uint32(harnessSnapshot.mode.Perm())
	}
	defer func() {
		if restoreErr := restoreLocalFile(harnessPath, harnessSnapshot); restoreErr != nil {
			record.Harness.RestorationError = restoreErr.Error()
			record.Error = appendError(record.Error, "restore harness: "+restoreErr.Error())
			return
		}
		if harnessSnapshot.existed {
			body, readErr := os.ReadFile(harnessPath)
			record.Harness.Restored = readErr == nil && bytes.Equal(body, harnessSnapshot.body)
			if !record.Harness.Restored {
				record.Error = appendError(record.Error, "pre-existing harness was not restored byte-exactly")
			}
		} else {
			_, statErr := os.Lstat(harnessPath)
			record.Harness.Removed = os.IsNotExist(statErr)
			if !record.Harness.Removed {
				record.Error = appendError(record.Error, "generated harness was not removed")
			}
		}
	}()
	if err := writeExact(harnessPath, prepared.plan.Evidence.Harness, 0o700); err != nil {
		record.Error = "stage harness: " + err.Error()
		return record
	}
	staged, err := os.ReadFile(harnessPath)
	if err != nil || digestBytes(staged) != prepared.plan.Evidence.HarnessDigest {
		record.Error = "staged harness bytes differ from frozen evidence"
		return record
	}
	record.Harness.StagedSHA256 = digestBytes(staged)

	generated := map[string]*ExhaustiveGeneratedOutputEvidence{}
	defer func() {
		ids := make([]string, 0, len(generated))
		for id := range generated {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			value := generated[id]
			path := filepath.Join(runRoot, filepath.FromSlash(value.Path))
			removeErr := os.Remove(path)
			if removeErr != nil && !os.IsNotExist(removeErr) {
				value.RemovalError = removeErr.Error()
				record.Error = appendError(record.Error, fmt.Sprintf("remove generated output %q: %v", id, removeErr))
			} else {
				_, statErr := os.Lstat(path)
				value.RemovedAfterRun = os.IsNotExist(statErr)
			}
			record.GeneratedOutputs = append(record.GeneratedOutputs, *value)
		}
	}()

	for _, step := range prepared.setup {
		for _, output := range step.Outputs {
			declared := prepared.outputs[output.ID]
			path := filepath.Join(runRoot, declared.pathRel)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				record.Error = fmt.Sprintf("create output parent for %q: %v", output.ID, err)
				return record
			}
			if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
				record.Error = fmt.Sprintf("generated output %q was not fresh", output.ID)
				return record
			}
		}
		command := runExhaustiveReplayStep(ctx, prepared, runRoot, step, "")
		record.Setup = append(record.Setup, command)
		if !command.Passed {
			record.Error = fmt.Sprintf("setup step %q did not reproduce its declared process observation: %s", step.ID, command.Error)
			return record
		}
		for _, output := range step.Outputs {
			declared := prepared.outputs[output.ID]
			path := filepath.Join(runRoot, declared.pathRel)
			inspected, inspectErr := inspectReplayOutput(path, declared.pathRel, output, step.ID)
			if inspectErr != nil {
				record.Error = inspectErr.Error()
				return record
			}
			generated[output.ID] = &inspected
		}
	}

	step, exists := prepared.runs[observation.StepID]
	if !exists {
		record.Error = fmt.Sprintf("declared run step %q is missing", observation.StepID)
		return record
	}
	generatedPath := ""
	if step.GeneratedExecutableID != "" {
		output, exists := generated[step.GeneratedExecutableID]
		if !exists || !output.Output.Executable {
			record.Error = fmt.Sprintf("run step %q has no fresh executable output", step.ID)
			return record
		}
		if err := verifyReplayOutput(runRoot, output, true); err != nil {
			record.Error = err.Error()
			return record
		}
		generatedPath = filepath.Join(runRoot, filepath.FromSlash(output.Path))
	}
	record.Run = runExhaustiveReplayStep(ctx, prepared, runRoot, step, generatedPath)
	if step.GeneratedExecutableID != "" {
		if err := verifyReplayOutput(runRoot, generated[step.GeneratedExecutableID], false); err != nil {
			record.Error = err.Error()
			return record
		}
	}
	if !record.Run.Passed {
		record.Error = fmt.Sprintf("run step %q did not reproduce its declared process observation: %s", step.ID, record.Run.Error)
		return record
	}
	actualTrace, traceErr := decodeRawOutcomeSignal(record.Run.SignalValue)
	if traceErr != nil || !reflect.DeepEqual(actualTrace, observation.RawOutcome) {
		if traceErr != nil {
			record.Error = "decode actual raw outcome signal: " + traceErr.Error()
		} else {
			record.Error = "actual raw outcome facts differ from the declared trace"
		}
		return record
	}
	if record.Run.ExitCode == nil || *record.Run.ExitCode != observation.ExitCode || !bytes.Equal(record.Run.Stdout, observation.Stdout) || !bytes.Equal(record.Run.Stderr, observation.Stderr) || !bytes.Equal(record.Run.SignalValue, observation.SignalValue) {
		record.Error = "actual exit/stdout/stderr/signal differs from embedded raw observation"
		return record
	}
	record.OutputsMatch = true
	record.FreshProcess = !record.Run.StartedAt.IsZero() && record.Run.Duration > 0

	for _, step := range prepared.cleanup {
		command := runExhaustiveReplayStep(ctx, prepared, runRoot, step, "")
		record.Cleanup = append(record.Cleanup, command)
		if !command.Passed {
			record.Error = fmt.Sprintf("cleanup step %q failed: %s", step.ID, command.Error)
			return record
		}
	}
	record.ToolsVerifiedAfterRun = verifyPreparedProbeTools(prepared.tools) == nil
	if !record.ToolsVerifiedAfterRun {
		record.Error = "frozen tool binding changed during replay"
	}
	return record
}

func inspectReplayOutput(path, pathRel string, declared semanticir.ProbeOutput, producer string) (ExhaustiveGeneratedOutputEvidence, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ExhaustiveGeneratedOutputEvidence{}, fmt.Errorf("setup output %q is not a fresh regular non-symlink file", declared.ID)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ExhaustiveGeneratedOutputEvidence{}, fmt.Errorf("read setup output %q: %w", declared.ID, err)
	}
	digest := digestBytes(body)
	if digest != declared.AfterDigest || declared.Executable && info.Mode().Perm()&0o111 == 0 {
		return ExhaustiveGeneratedOutputEvidence{}, fmt.Errorf("setup output %q bytes or executable mode differ from declaration", declared.ID)
	}
	return ExhaustiveGeneratedOutputEvidence{
		Output: declared, ProducerStepID: producer, Path: filepath.ToSlash(pathRel), Fresh: true,
		SHA256: digest, Mode: uint32(info.Mode().Perm()), Size: int64(len(body)),
	}, nil
}

func verifyReplayOutput(runRoot string, output *ExhaustiveGeneratedOutputEvidence, before bool) error {
	path := output.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(runRoot, filepath.FromSlash(path))
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("generated executable %q is no longer a regular file", output.Output.ID)
	}
	body, err := os.ReadFile(path)
	if err != nil || digestBytes(body) != output.SHA256 || uint32(info.Mode().Perm()) != output.Mode || int64(len(body)) != output.Size {
		return fmt.Errorf("generated executable %q changed after setup", output.Output.ID)
	}
	if before {
		output.VerifiedBeforeRun, output.BeforeRunSHA256 = true, digestBytes(body)
	} else {
		output.VerifiedAfterRun, output.AfterRunSHA256 = true, digestBytes(body)
	}
	return nil
}

func runExhaustiveReplayStep(ctx context.Context, prepared preparedExhaustiveReplay, runRoot string, step semanticir.ProbeStep, generatedPath string) (record ExhaustiveReplayCommandEvidence) {
	record.Step = step
	record.StepSHA256, _ = semanticir.Digest(step)
	record.Argv = append([]string(nil), step.Argv...)
	record.StdinSHA256 = digestBytes(step.Stdin)
	record.StdoutSHA256, record.StderrSHA256, record.SignalSHA256 = digestBytes(nil), digestBytes(nil), digestBytes(nil)
	record.EnvironmentSHA256, _ = semanticir.Digest(step.Environment)
	workRel, err := replayWorkRelative(prepared.root, step.WorkingDirectory)
	if err != nil {
		record.Error = err.Error()
		return record
	}
	record.WorkingDirectory = filepath.Join(runRoot, workRel)
	if generatedPath != "" {
		record.ExecutablePath = generatedPath
		body, readErr := os.ReadFile(generatedPath)
		if readErr != nil {
			record.Error = readErr.Error()
			return record
		}
		record.ExecutableSHA256 = digestBytes(body)
	} else {
		toolPath, resolveErr := filepath.EvalSymlinks(step.Tool.Path)
		if resolveErr != nil {
			record.Error = resolveErr.Error()
			return record
		}
		record.ExecutablePath, record.ExecutableSHA256 = filepath.Clean(toolPath), step.Tool.Digest
	}
	arguments := append([]string(nil), step.Argv...)
	for index, argument := range arguments {
		if filepath.IsAbs(argument) && pathWithin(prepared.root, filepath.Clean(argument)) {
			relative, _ := filepath.Rel(prepared.root, filepath.Clean(argument))
			arguments[index] = filepath.Join(runRoot, relative)
		}
	}
	environment := make([]string, 0, len(step.Environment))
	for _, variable := range step.Environment {
		environment = append(environment, variable.Name+"="+variable.Value)
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(step.TimeoutMillis)*time.Millisecond)
	defer cancel()
	if err := runCtx.Err(); err != nil {
		record.Interrupted = !errors.Is(err, context.DeadlineExceeded)
		record.TimedOut = errors.Is(err, context.DeadlineExceeded)
		record.Error = err.Error()
		return record
	}
	signalPath := ""
	if step.SignalExtractor.Kind == semanticir.ProbeSignalRawOutcomeFile {
		declaredSignal, signalErr := replayOutputRelative(runRoot, workRel, step.SignalExtractor.Path)
		if signalErr != nil {
			record.Error = "signal path: " + signalErr.Error()
			return record
		}
		signalPath = filepath.Join(runRoot, declaredSignal)
		if mkdirErr := os.MkdirAll(filepath.Dir(signalPath), 0o750); mkdirErr != nil {
			record.Error = "create signal parent: " + mkdirErr.Error()
			return record
		}
		if _, statErr := os.Lstat(signalPath); !os.IsNotExist(statErr) {
			record.Error = "raw-outcome signal file was not fresh"
			return record
		}
		record.SignalPath = filepath.ToSlash(declaredSignal)
	}
	command := exec.Command(record.ExecutablePath, arguments...)
	command.Dir = record.WorkingDirectory
	command.Env = environment
	command.Stdin = bytes.NewReader(step.Stdin)
	command.WaitDelay = processWaitDelay
	configureProcess(command)
	stdout, stderr := &limitedBuffer{limit: maxCapturedOutput}, &limitedBuffer{limit: maxCapturedOutput}
	command.Stdout, command.Stderr = stdout, stderr
	record.StartedAt = time.Now().UTC()
	started := time.Now()
	if err := command.Start(); err != nil {
		record.Duration, record.Error = time.Since(started), "start process: "+err.Error()
		return record
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var waitErr error
	select {
	case waitErr = <-done:
		terminateProcess(command)
	case <-runCtx.Done():
		terminateProcess(command)
		waitErr = <-done
		record.TimedOut = errors.Is(runCtx.Err(), context.DeadlineExceeded)
		record.Interrupted = !record.TimedOut
		record.Error = runCtx.Err().Error()
	}
	record.Duration = time.Since(started)
	record.Stdout, record.Stderr = append([]byte(nil), stdout.buffer.Bytes()...), append([]byte(nil), stderr.buffer.Bytes()...)
	record.StdoutSHA256, record.StderrSHA256 = digestBytes(record.Stdout), digestBytes(record.Stderr)
	record.OutputTruncated = stdout.truncated || stderr.truncated
	if command.ProcessState != nil {
		code := command.ProcessState.ExitCode()
		record.ExitCode = &code
	}
	if record.Error == "" && waitErr != nil {
		var exitError *exec.ExitError
		if !errors.As(waitErr, &exitError) {
			record.Error = "wait process: " + waitErr.Error()
		}
	}
	if record.ExitCode == nil || record.OutputTruncated {
		if record.Error == "" {
			record.Error = "process has no exact bounded output"
		}
		return record
	}
	signal, signalOK, signalErr := observedReplaySignal(step.SignalExtractor, signalPath, record.Stdout)
	if signalErr != nil {
		record.Error = signalErr.Error()
		return record
	}
	record.SignalValue, record.SignalSHA256 = signal, digestBytes(signal)
	record.FreshSignal = step.SignalExtractor.Kind == semanticir.ProbeSignalNone || signalOK
	if signalPath != "" {
		removeErr := os.Remove(signalPath)
		record.SignalRemoved = removeErr == nil
		if removeErr != nil {
			record.Error = "remove raw-outcome signal file: " + removeErr.Error()
			return record
		}
	} else {
		record.SignalRemoved = true
	}
	record.Passed = record.Error == "" && signalOK && *record.ExitCode == step.ExpectedExitCode && record.StdoutSHA256 == step.ExpectedStdoutDigest && record.StderrSHA256 == step.ExpectedStderrDigest && record.SignalSHA256 == step.ExpectedSignalDigest
	if !record.Passed && record.Error == "" {
		record.Error = "actual process output differs from the exact declared exit/stdout/stderr/signal"
	}
	return record
}

func observedReplaySignal(extractor semanticir.ProbeSignalExtractor, signalPath string, stdout []byte) ([]byte, bool, error) {
	switch extractor.Kind {
	case semanticir.ProbeSignalNone:
		return nil, true, nil
	case semanticir.ProbeSignalRawOutcomeStdout:
		return append([]byte(nil), stdout...), true, nil
	case semanticir.ProbeSignalRawOutcomeFile:
		info, err := os.Lstat(signalPath)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxCapturedOutput {
			return nil, false, fmt.Errorf("raw-outcome signal is not a fresh bounded regular file")
		}
		body, err := os.ReadFile(signalPath)
		if err != nil {
			return nil, false, fmt.Errorf("read raw-outcome signal: %w", err)
		}
		return body, true, nil
	default:
		return nil, false, fmt.Errorf("unsupported raw-outcome signal extractor %q", extractor.Kind)
	}
}

func decodeRawOutcomeSignal(body []byte) (semanticir.RawOutcomeTrace, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var trace semanticir.RawOutcomeTrace
	if err := decoder.Decode(&trace); err != nil {
		return semanticir.RawOutcomeTrace{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return semanticir.RawOutcomeTrace{}, fmt.Errorf("raw outcome signal contains trailing JSON")
	}
	if err := semanticir.ValidateRawOutcomeTrace(trace); err != nil {
		return semanticir.RawOutcomeTrace{}, err
	}
	canonical, err := semanticir.CanonicalJSON(trace)
	if err != nil || !bytes.Equal(canonical, body) {
		return semanticir.RawOutcomeTrace{}, fmt.Errorf("raw outcome signal is not canonical JSON")
	}
	return trace, nil
}

// SemanticReplay returns the compact Semantic IR transcript proven by a
// confirmed executor replay. Pipeline code should use this function rather
// than reconstructing cleanup paths or digests from executor internals.
func SemanticReplay(evidence ExhaustiveReplayEvidence) (semanticir.ExhaustiveReplayEvidence, error) {
	if evidence.Status != StatusConfirmed {
		return semanticir.ExhaustiveReplayEvidence{}, fmt.Errorf("exhaustive replay is not confirmed")
	}
	want, err := semanticReplayForPlan(evidence.Plan)
	if err != nil {
		return semanticir.ExhaustiveReplayEvidence{}, err
	}
	if !reflect.DeepEqual(evidence.SemanticReplay, want) {
		return semanticir.ExhaustiveReplayEvidence{}, fmt.Errorf("retained semantic replay differs from confirmed plan")
	}
	digest, err := semanticir.Digest(want)
	if err != nil || evidence.SemanticReplaySHA256 != digest {
		return semanticir.ExhaustiveReplayEvidence{}, fmt.Errorf("semantic replay digest is inconsistent")
	}
	return want, nil
}

func semanticReplayForPlan(plan ExhaustiveReplayPlan) (semanticir.ExhaustiveReplayEvidence, error) {
	root, err := filepath.EvalSymlinks(plan.Workspace.Root)
	if err != nil {
		return semanticir.ExhaustiveReplayEvidence{}, fmt.Errorf("resolve replay workspace: %w", err)
	}
	cleanupSet := map[string]bool{}
	addCleanup := func(path string) error {
		path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
		if path == "" || path == "." || filepath.IsAbs(filepath.FromSlash(path)) || path == ".." || strings.HasPrefix(filepath.FromSlash(path), ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe replay cleanup path %q", path)
		}
		cleanupSet[path] = true
		return nil
	}
	if err := addCleanup(plan.Evidence.HarnessPath); err != nil {
		return semanticir.ExhaustiveReplayEvidence{}, err
	}
	var generated []semanticir.ProbeOutput
	for _, step := range plan.Evidence.Steps {
		workRel, workErr := replayWorkRelative(root, step.WorkingDirectory)
		if workErr != nil {
			return semanticir.ExhaustiveReplayEvidence{}, fmt.Errorf("step %q workdir: %w", step.ID, workErr)
		}
		for _, output := range step.Outputs {
			path, pathErr := replayOutputRelative(root, workRel, output.Path)
			if pathErr != nil {
				return semanticir.ExhaustiveReplayEvidence{}, fmt.Errorf("step %q output: %w", step.ID, pathErr)
			}
			if err := addCleanup(path); err != nil {
				return semanticir.ExhaustiveReplayEvidence{}, err
			}
			generated = append(generated, output)
		}
		if step.SignalExtractor.Kind == semanticir.ProbeSignalRawOutcomeFile {
			path, pathErr := replayOutputRelative(root, workRel, step.SignalExtractor.Path)
			if pathErr != nil {
				return semanticir.ExhaustiveReplayEvidence{}, fmt.Errorf("step %q signal: %w", step.ID, pathErr)
			}
			if err := addCleanup(path); err != nil {
				return semanticir.ExhaustiveReplayEvidence{}, err
			}
		}
	}
	cleanupPaths := make([]string, 0, len(cleanupSet))
	for path := range cleanupSet {
		cleanupPaths = append(cleanupPaths, path)
	}
	sort.Strings(cleanupPaths)
	coreDigest, err := semanticir.ExhaustiveExecutionCoreDigest(plan.Evidence)
	if err != nil {
		return semanticir.ExhaustiveReplayEvidence{}, err
	}
	stepsDigest, err := semanticir.Digest(plan.Evidence.Steps)
	if err != nil {
		return semanticir.ExhaustiveReplayEvidence{}, err
	}
	cleanupDigest, err := semanticir.Digest(cleanupPaths)
	if err != nil {
		return semanticir.ExhaustiveReplayEvidence{}, err
	}
	return semanticir.ExhaustiveReplayEvidence{
		CoreDigest: coreDigest, StepsDigest: stepsDigest,
		Runs:             append([]semanticir.ExecutionRunEvidence(nil), plan.Evidence.Runs...),
		GeneratedOutputs: generated, CleanupPaths: cleanupPaths, CleanupDigest: cleanupDigest,
		Clean: true, Provenance: plan.Evidence.Provenance,
	}, nil
}

// ValidateExhaustiveReplay checks the self-contained certificate-facing
// record. Callers must additionally cross-bind Plan to the authoritative
// frozen manifest and ArtifactModel.
func ValidateExhaustiveReplay(evidence ExhaustiveReplayEvidence) error {
	if evidence.Status != StatusConfirmed || len(evidence.Blockers) != 0 || len(evidence.Runs) != 2 || !evidence.OriginalWorkspaceIntact {
		return fmt.Errorf("exhaustive replay is not a complete confirmed record")
	}
	planDigest, _ := semanticir.Digest(evidence.Plan)
	executionDigest, _ := semanticir.Digest(evidence.Plan.Evidence)
	if evidence.PlanSHA256 != planDigest || evidence.ExecutionSHA256 != executionDigest || evidence.WorkspaceSHA256 != evidence.Plan.Workspace.TreeSHA256 || evidence.OriginalBeforeSHA256 != evidence.WorkspaceSHA256 || evidence.OriginalAfterSHA256 != evidence.WorkspaceSHA256 {
		return fmt.Errorf("exhaustive replay top-level digests are inconsistent")
	}
	if _, err := SemanticReplay(evidence); err != nil {
		return err
	}
	if len(evidence.SourceBindings) != len(evidence.Plan.SourceArtifacts) || len(evidence.ToolBindings) == 0 {
		return fmt.Errorf("exhaustive replay omits immutable bindings")
	}
	for index, binding := range evidence.SourceBindings {
		source := evidence.Plan.SourceArtifacts[index]
		if binding.ID != source.ID || binding.ExpectedSHA256 != source.Digest {
			return fmt.Errorf("exhaustive replay source binding %d differs from its plan", index)
		}
	}
	for _, binding := range append(append([]BindingEvidence(nil), evidence.SourceBindings...), evidence.ToolBindings...) {
		if !binding.Verified || !validDigest(binding.ExpectedSHA256) || binding.ExpectedSHA256 != binding.ObservedSHA256 {
			return fmt.Errorf("exhaustive replay has an unverified immutable binding")
		}
	}
	setupSteps, runSteps, declaredOutputs, err := exhaustiveReplayDeclarations(evidence.Plan)
	if err != nil {
		return err
	}
	for runIndex, run := range evidence.Runs {
		if !reflect.DeepEqual(run.Run, evidence.Plan.Evidence.Runs[runIndex]) || run.RunSHA256 != mustProbeDigest(run.Run) || run.OrderDigest != run.Run.OrderDigest || run.ObservationDigest != run.Run.ObservationDigest || !run.Complete || len(run.Observations) != len(run.Run.Observations) {
			return fmt.Errorf("exhaustive replay run %d differs from its retained declaration", runIndex)
		}
		for index, observation := range run.Observations {
			if !reflect.DeepEqual(observation.Expected, run.Run.Observations[index]) || observation.ExpectedSHA256 != mustProbeDigest(observation.Expected) || observation.Error != "" || !observation.OutputsMatch || !observation.FreshProcess || !observation.ToolsVerifiedAfterRun {
				return fmt.Errorf("exhaustive replay observation %d/%d is incomplete", runIndex, index)
			}
			if len(observation.Setup) != len(setupSteps) || len(observation.Cleanup) != 0 || len(observation.GeneratedOutputs) != len(declaredOutputs) {
				return fmt.Errorf("exhaustive replay observation %d/%d omits setup or generated-output evidence", runIndex, index)
			}
			for setupIndex, command := range observation.Setup {
				if !reflect.DeepEqual(command.Step, setupSteps[setupIndex]) || command.StepSHA256 != mustProbeDigest(command.Step) || !command.Passed || command.Error != "" {
					return fmt.Errorf("exhaustive replay observation %d/%d setup %d differs from its plan", runIndex, index, setupIndex)
				}
			}
			declaredRun, exists := runSteps[observation.Expected.StepID]
			if !exists || !reflect.DeepEqual(observation.Run.Step, declaredRun) || observation.Run.StepSHA256 != mustProbeDigest(declaredRun) || observation.Run.ExitCode == nil || *observation.Run.ExitCode != observation.Expected.ExitCode || !bytes.Equal(observation.Run.Stdout, observation.Expected.Stdout) || !bytes.Equal(observation.Run.Stderr, observation.Expected.Stderr) || !bytes.Equal(observation.Run.SignalValue, observation.Expected.SignalValue) || !observation.Run.Passed || !observation.Run.FreshSignal || !observation.Run.SignalRemoved {
				return fmt.Errorf("exhaustive replay observation %d/%d process bytes differ", runIndex, index)
			}
			if !observation.Isolation.IsolatedRemoved || !observation.Isolation.OriginalIntact || observation.Isolation.Error != "" || observation.Isolation.CopyBeforeSHA256 != evidence.WorkspaceSHA256 {
				return fmt.Errorf("exhaustive replay observation %d/%d isolation is incomplete", runIndex, index)
			}
			if observation.Harness.StagedSHA256 != evidence.Plan.Evidence.HarnessDigest || observation.Harness.RestorationError != "" || observation.Harness.PreviouslyExisted && !observation.Harness.Restored || !observation.Harness.PreviouslyExisted && !observation.Harness.Removed {
				return fmt.Errorf("exhaustive replay observation %d/%d harness cleanup is incomplete", runIndex, index)
			}
			seenOutputs := map[string]bool{}
			for _, output := range observation.GeneratedOutputs {
				declared, exists := declaredOutputs[output.Output.ID]
				if !exists || seenOutputs[output.Output.ID] || !reflect.DeepEqual(output.Output, declared.output) || output.ProducerStepID != declared.producerID || output.Path != filepath.ToSlash(declared.pathRel) || !output.Fresh || output.SHA256 != output.Output.AfterDigest || output.Output.Executable && (!output.VerifiedBeforeRun || !output.VerifiedAfterRun || output.BeforeRunSHA256 != output.SHA256 || output.AfterRunSHA256 != output.SHA256) || !output.RemovedAfterRun || output.RemovalError != "" {
					return fmt.Errorf("exhaustive replay observation %d/%d generated output is incomplete", runIndex, index)
				}
				seenOutputs[output.Output.ID] = true
			}
		}
	}
	return nil
}

func exhaustiveReplayDeclarations(plan ExhaustiveReplayPlan) ([]semanticir.ProbeStep, map[string]semanticir.ProbeStep, map[string]preparedReplayOutput, error) {
	var setup []semanticir.ProbeStep
	runs := map[string]semanticir.ProbeStep{}
	outputs := map[string]preparedReplayOutput{}
	paths := map[string]bool{}
	harness := filepath.ToSlash(filepath.Clean(filepath.FromSlash(plan.Evidence.HarnessPath)))
	for _, step := range plan.Evidence.Steps {
		switch step.Kind {
		case semanticir.ProbeStepSetup:
			setup = append(setup, step)
		case semanticir.ProbeStepRun:
			if _, duplicate := runs[step.ID]; duplicate {
				return nil, nil, nil, fmt.Errorf("exhaustive replay repeats run step %q", step.ID)
			}
			runs[step.ID] = step
		default:
			return nil, nil, nil, fmt.Errorf("exhaustive replay contains a non-executable step kind")
		}
		for _, output := range step.Outputs {
			path, err := declaredReplayOutputPath(plan.Workspace.Root, step.WorkingDirectory, output.Path)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("exhaustive replay output %q: %w", output.ID, err)
			}
			if _, duplicate := outputs[output.ID]; duplicate || paths[path] || path == harness {
				return nil, nil, nil, fmt.Errorf("exhaustive replay generated output identities and paths are not unique")
			}
			outputs[output.ID] = preparedReplayOutput{output: output, producerID: step.ID, pathRel: path}
			paths[path] = true
		}
	}
	return setup, runs, outputs, nil
}
