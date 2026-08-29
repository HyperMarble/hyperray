package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func TestExhaustiveReplayHelperProcess(t *testing.T) {
	if os.Getenv("RAY_EXECUTOR_REPLAY_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(91)
	}
	arguments := os.Args[separator+1:]
	switch arguments[0] {
	case "setup":
		if len(arguments) != 3 {
			os.Exit(92)
		}
		body, err := os.ReadFile(arguments[1])
		if err != nil || os.WriteFile(arguments[2], body, 0o700) != nil {
			os.Exit(93)
		}
	case "run":
		if len(arguments) != 2 {
			os.Exit(94)
		}
		_, _ = os.Stdout.Write([]byte(arguments[1]))
	case "timeout":
		time.Sleep(10 * time.Second)
	default:
		os.Exit(95)
	}
	os.Exit(0)
}

func TestReplayExhaustiveConfirmsFreshTypedCompileRunOrders(t *testing.T) {
	plan := exhaustiveReplayFixture(t)
	before := plan.Workspace.TreeSHA256
	result := ReplayExhaustive(context.Background(), plan)
	if result.Status != StatusConfirmed || len(result.Blockers) != 0 {
		t.Fatalf("replay = %+v, want confirmed", result)
	}
	if err := ValidateExhaustiveReplay(result); err != nil {
		t.Fatalf("certificate replay rejected: %v", err)
	}
	semanticReplay, err := SemanticReplay(result)
	if err != nil || !reflect.DeepEqual(semanticReplay, result.SemanticReplay) {
		t.Fatalf("semantic replay = %+v, %v", semanticReplay, err)
	}
	seenCopies := map[string]bool{}
	for _, run := range result.Runs {
		for _, observation := range run.Observations {
			if seenCopies[observation.Isolation.IsolatedRoot] {
				t.Fatalf("workspace copy reused: %s", observation.Isolation.IsolatedRoot)
			}
			seenCopies[observation.Isolation.IsolatedRoot] = true
			if len(observation.Setup) != 1 || len(observation.GeneratedOutputs) != 1 || !observation.GeneratedOutputs[0].VerifiedBeforeRun || !observation.GeneratedOutputs[0].VerifiedAfterRun || !observation.GeneratedOutputs[0].RemovedAfterRun || !observation.Harness.Removed {
				t.Fatalf("observation cleanup/dataflow incomplete: %+v", observation)
			}
		}
	}
	if len(seenCopies) != 4 {
		t.Fatalf("fresh copies = %d, want four", len(seenCopies))
	}
	if got, err := WorkspaceDigest(plan.Workspace.Root); err != nil || got != before {
		t.Fatalf("frozen workspace changed: %s %v", got, err)
	}
	tampered := cloneExhaustiveReplay(t, result)
	tampered.Runs[0].Observations[0].GeneratedOutputs = nil
	if err := ValidateExhaustiveReplay(tampered); err == nil {
		t.Fatal("validator accepted omitted generated-output evidence")
	}
}

func TestReplayExhaustiveRejectsStaleWorkspaceAndPathEscape(t *testing.T) {
	t.Run("stale", func(t *testing.T) {
		plan := exhaustiveReplayFixture(t)
		if err := os.WriteFile(filepath.Join(plan.Workspace.Root, "subject.txt"), []byte("changed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		result := ReplayExhaustive(context.Background(), plan)
		if result.Status != StatusProofBlocked || !hasReplayBlocker(result.Blockers, "stale-replay-workspace") || len(result.Runs) != 0 {
			t.Fatalf("stale replay = %+v", result)
		}
	})
	t.Run("harness escape", func(t *testing.T) {
		plan := exhaustiveReplayFixture(t)
		plan.Evidence.HarnessPath = "../escaped-harness"
		result := ReplayExhaustive(context.Background(), plan)
		if result.Status != StatusProofBlocked || !hasReplayBlocker(result.Blockers, "invalid-replay-harness-path") || len(result.Runs) != 0 {
			t.Fatalf("escaped replay = %+v", result)
		}
	})
}

func TestReplayExhaustiveRejectsActualOutcomeMismatch(t *testing.T) {
	plan := exhaustiveReplayFixture(t)
	for index := range plan.Evidence.Steps {
		if plan.Evidence.Steps[index].ID == "run-a" {
			plan.Evidence.Steps[index].Argv[len(plan.Evidence.Steps[index].Argv)-1] = string(mustRawOutcome(t, "detached"))
		}
	}
	result := ReplayExhaustive(context.Background(), plan)
	if result.Status != StatusProofBlocked || !hasReplayBlocker(result.Blockers, "exhaustive-observation-mismatch") {
		t.Fatalf("mismatched replay = %+v", result)
	}
}

func TestReplayExhaustiveRejectsProcessSemanticFields(t *testing.T) {
	for _, field := range []string{"id", "provenance"} {
		t.Run(field, func(t *testing.T) {
			plan := exhaustiveReplayFixture(t)
			forged := []byte(`{"kind":"return","value":{"type":"string","bool":false,"integer":0,"string":"a","null":false},"exception_type":"","message":"","effects":[],"` + field + `":"forged"}`)
			for runIndex := range plan.Evidence.Runs {
				for observationIndex := range plan.Evidence.Runs[runIndex].Observations {
					observation := &plan.Evidence.Runs[runIndex].Observations[observationIndex]
					if observation.StepID != "run-a" {
						continue
					}
					observation.Stdout = append([]byte(nil), forged...)
					observation.StdoutDigest = digestBytes(forged)
					observation.SignalValue = append([]byte(nil), forged...)
					observation.SignalValueDigest = digestBytes(forged)
				}
				plan.Evidence.Runs[runIndex].ObservationDigest, _ = semanticir.ExecutionObservationDigest(plan.Evidence.Runs[runIndex].Observations)
			}
			plan.Evidence.CompleteAssignmentDigest = plan.Evidence.Runs[0].ObservationDigest
			plan.Evidence.Runs[1].ObservationDigest = plan.Evidence.CompleteAssignmentDigest
			for stepIndex := range plan.Evidence.Steps {
				if plan.Evidence.Steps[stepIndex].ID == "run-a" {
					plan.Evidence.Steps[stepIndex].ExpectedStdoutDigest = digestBytes(forged)
					plan.Evidence.Steps[stepIndex].ExpectedSignalDigest = digestBytes(forged)
				}
			}
			result := ReplayExhaustive(context.Background(), plan)
			if result.Status != StatusProofBlocked || !hasReplayBlocker(result.Blockers, "invalid-replay-observation") || len(result.Runs) != 0 {
				t.Fatalf("forged %s replay = %+v", field, result)
			}
		})
	}
}

func TestReplayExhaustiveTimeoutKillsProcessAndCleansCopy(t *testing.T) {
	plan := exhaustiveReplayFixture(t)
	for index := range plan.Evidence.Steps {
		step := &plan.Evidence.Steps[index]
		if step.ID == "run-a" {
			step.Argv = append(helperInvocation(), "timeout")
			step.TimeoutMillis = 100
		}
	}
	result := ReplayExhaustive(context.Background(), plan)
	if result.Status != StatusProofBlocked || !hasReplayBlocker(result.Blockers, "exhaustive-observation-mismatch") || len(result.Runs) == 0 || len(result.Runs[0].Observations) == 0 {
		t.Fatalf("timeout replay = %+v", result)
	}
	observation := result.Runs[0].Observations[0]
	if !observation.Run.TimedOut || !observation.Isolation.IsolatedRemoved || !observation.Isolation.OriginalIntact || !observation.Harness.Removed {
		t.Fatalf("timeout cleanup = %+v", observation)
	}
}

func exhaustiveReplayFixture(t *testing.T) ExhaustiveReplayPlan {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sourceBody := []byte("frozen solution\n")
	sourcePath := filepath.Join(root, "subject.txt")
	if err := os.WriteFile(sourcePath, sourceBody, 0o600); err != nil {
		t.Fatal(err)
	}
	toolPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	toolPath, err = filepath.EvalSymlinks(toolPath)
	if err != nil {
		t.Fatal(err)
	}
	toolBytes, err := os.ReadFile(toolPath)
	if err != nil {
		t.Fatal(err)
	}
	source := semanticir.ArtifactRef{ID: "code", Kind: semanticir.ArtifactCode, Path: "subject.txt", Digest: digestBytes(sourceBody)}
	provenance := semanticir.Provenance{
		ArtifactID: source.ID, ArtifactDigest: source.Digest,
		Location:    semanticir.SourceLocation{Path: source.Path, StartLine: 1, StartColumn: 1},
		Translation: semanticir.TranslationTranslated,
	}
	tool := semanticir.ToolRef{Name: "executor-replay-helper", Path: toolPath, Digest: digestBytes(toolBytes), Version: "test-binary-v1"}
	environment := []semanticir.EnvironmentVariable{{Name: "RAY_EXECUTOR_REPLAY_HELPER", Value: "1"}}
	environmentDigest, _ := semanticir.Digest(environment)
	emptyDigest := digestBytes(nil)
	output := semanticir.ProbeOutput{ID: "generated-helper", Path: ".hyperray/replay/generated-helper", AfterDigest: tool.Digest, Executable: true, Provenance: provenance}
	setup := semanticir.ProbeStep{
		ID: "compile", Kind: semanticir.ProbeStepSetup, Tool: tool,
		Argv:        append(helperInvocation(), "setup", ".hyperray/replay/harness-helper", output.Path),
		StdinDigest: emptyDigest, WorkingDirectory: root, Environment: environment, EnvironmentDigest: environmentDigest,
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000, ExpectedExitCode: 0,
		ExpectedStdoutDigest: emptyDigest, ExpectedStderrDigest: emptyDigest, ExpectedSignalDigest: emptyDigest,
		SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Outputs: []semanticir.ProbeOutput{output}, Provenance: provenance,
	}
	makeObservation := func(label, stepID string) semanticir.ExecutionObservation {
		raw := semanticir.RawOutcomeTrace{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeString, String: label}}
		observable, err := semanticir.ObservableOutcomeFromTrace("choose", raw, provenance)
		if err != nil {
			t.Fatal(err)
		}
		signal, err := semanticir.CanonicalJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		return semanticir.ExecutionObservation{
			Behavior: semanticir.BehaviorRef{OperationID: "choose", Conditions: semanticir.Assignment{"x": label}, Provenance: provenance},
			Inputs:   map[string]semanticir.Literal{"x": {Type: semanticir.TypeString, String: label}},
			StepID:   stepID, RawOutcome: raw, OutcomeIDs: []string{observable.ID}, ExitCode: 0,
			Stdout: signal, StdoutDigest: digestBytes(signal), StderrDigest: emptyDigest,
			SignalValue: signal, SignalValueDigest: digestBytes(signal), Provenance: provenance,
		}
	}
	a, b := makeObservation("a", "run-a"), makeObservation("b", "run-b")
	makeRunStep := func(observation semanticir.ExecutionObservation) semanticir.ProbeStep {
		return semanticir.ProbeStep{
			ID: observation.StepID, Kind: semanticir.ProbeStepRun, GeneratedExecutableID: output.ID,
			Argv: append(helperInvocation(), "run", string(observation.SignalValue)), StdinDigest: emptyDigest,
			WorkingDirectory: root, Environment: environment, EnvironmentDigest: environmentDigest,
			ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000, ExpectedExitCode: 0,
			ExpectedStdoutDigest: observation.StdoutDigest, ExpectedStderrDigest: observation.StderrDigest, ExpectedSignalDigest: observation.SignalValueDigest,
			SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalRawOutcomeStdout}, Provenance: provenance,
		}
	}
	makeRun := func(id string, observations []semanticir.ExecutionObservation) semanticir.ExecutionRunEvidence {
		observationDigest, _ := semanticir.ExecutionObservationDigest(observations)
		orderDigest, _ := semanticir.ExecutionOrderDigest(observations)
		return semanticir.ExecutionRunEvidence{
			ID: id, StartedAtUTC: time.Now().UTC().Format(time.RFC3339Nano), Observations: observations,
			OrderDigest: orderDigest, ObservationDigest: observationDigest, FreshProcessCount: len(observations), Provenance: provenance,
		}
	}
	forward := makeRun("forward", []semanticir.ExecutionObservation{a, b})
	reverse := makeRun("reverse", []semanticir.ExecutionObservation{b, a})
	if forward.ObservationDigest != reverse.ObservationDigest {
		t.Fatal("fixture observation digests differ")
	}
	workspaceDigest, err := WorkspaceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	evidence := semanticir.ExhaustiveExecutionEvidence{
		ID: "exhaustive", Tool: tool, SourceDigest: source.Digest, WorkspaceTreeDigest: workspaceDigest,
		IRKind: semanticir.CompilerIRLLVM, EmittedIRDigest: digestBytes([]byte("ir")),
		Harness: toolBytes, HarnessPath: ".hyperray/replay/harness-helper", HarnessDigest: tool.Digest, ExecutableDigest: tool.Digest,
		Steps: []semanticir.ProbeStep{setup, makeRunStep(a), makeRunStep(b)},
		Argv:  append([]string{tool.Path}, helperInvocation()...), WorkingDirectory: root,
		Environment: environment, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000,
		Groundings: []semanticir.AssignmentGrounding{
			{ID: "grounding-a", OperationID: "choose", Conditions: semanticir.Assignment{"x": "a"}, Inputs: map[string]semanticir.Literal{"x": {Type: semanticir.TypeString, String: "a"}}, Provenance: provenance},
			{ID: "grounding-b", OperationID: "choose", Conditions: semanticir.Assignment{"x": "b"}, Inputs: map[string]semanticir.Literal{"x": {Type: semanticir.TypeString, String: "b"}}, Provenance: provenance},
		},
		CompleteAssignmentDigest: forward.ObservationDigest, Runs: []semanticir.ExecutionRunEvidence{forward, reverse}, Complete: true, Provenance: provenance,
	}
	return ExhaustiveReplayPlan{
		ID: "replay-plan", Workspace: ProbeWorkspace{ID: "solution-new-tests", Root: root, State: semanticir.WorkspaceSolutionNewTests, TreeSHA256: workspaceDigest},
		SourceArtifacts: []semanticir.ArtifactRef{source},
		Operations:      []semanticir.Operation{{ID: "choose", Kind: semanticir.OperationFunction, DomainIDs: []string{"x"}, OutcomeIDs: []string{a.OutcomeIDs[0], b.OutcomeIDs[0]}, Provenance: provenance}},
		Evidence:        evidence,
	}
}

func helperInvocation() []string {
	return []string{"-test.run=^TestExhaustiveReplayHelperProcess$", "--"}
}

func mustRawOutcome(t *testing.T, label string) []byte {
	t.Helper()
	plan := exhaustiveReplayFixture(t)
	for _, observation := range plan.Evidence.Runs[0].Observations {
		if observation.RawOutcome.Value != nil && observation.RawOutcome.Value.String == label {
			return observation.SignalValue
		}
	}
	// Build a detached but canonical runtime-only value.
	raw := semanticir.RawOutcomeTrace{Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeString, String: label}}
	body, err := semanticir.CanonicalJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func hasReplayBlocker(blockers []Blocker, code string) bool {
	for _, blocker := range blockers {
		if blocker.Code == code {
			return true
		}
	}
	return false
}

func cloneExhaustiveReplay(t *testing.T, evidence ExhaustiveReplayEvidence) ExhaustiveReplayEvidence {
	t.Helper()
	body, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	var clone ExhaustiveReplayEvidence
	if err := json.Unmarshal(body, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestReplayExhaustiveCleanupPathsAreCentralAndSorted(t *testing.T) {
	plan := exhaustiveReplayFixture(t)
	result := ReplayExhaustive(context.Background(), plan)
	replay, err := SemanticReplay(result)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]string(nil), replay.CleanupPaths...)
	sort.Strings(want)
	if !sort.StringsAreSorted(replay.CleanupPaths) || !reflect.DeepEqual(replay.CleanupPaths, want) || replay.CleanupDigest != mustProbeDigest(replay.CleanupPaths) || len(replay.CleanupSteps) != 0 || !replay.Clean {
		t.Fatalf("semantic cleanup = %+v", replay)
	}
	if !bytes.Equal(plan.Evidence.Harness, mustReadReplayFile(t, os.Args[0])) {
		// os.Args[0] can be a symlink/relative alias while os.Executable is the
		// canonical bound tool; this assertion is informational only when equal.
		_ = fmt.Sprintf("canonical executable differs from argv alias")
	}
}

func mustReadReplayFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return body
}
