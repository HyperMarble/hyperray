package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/ray/internal/executor"
	frontendpython "github.com/HyperMarble/ray/internal/frontend/python"
	"github.com/HyperMarble/ray/internal/semanticir"
)

func TestCounterexampleExecutorBaselineMustPassBeforeMaterialization(t *testing.T) {
	dir := t.TempDir()
	source := writeExecutorFixture(t, dir, "subject.txt", []byte("clean\n"))
	logPath := filepath.Join(dir, "runs.log")
	script := writeExecutorScript(t, dir, `
printf '%s\n' "$(tr -d '\n' < subject.txt)" >> runs.log
if grep -q '^clean$' subject.txt; then
  exit 23
fi
touch counterexample-was-executed
exit 0
`)

	plan := executorPlan("baseline-plan", source, []byte("clean\n"), []byte("changed\n"), true)
	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})

	if report.Status != executor.StatusProofBlocked {
		t.Fatalf("status = %q, want %q", report.Status, executor.StatusProofBlocked)
	}
	if report.Baseline.Passed {
		t.Fatal("failing clean baseline was recorded as passing")
	}
	if len(report.Confirmations) != 0 {
		t.Fatalf("%d counterexamples executed after a failed baseline", len(report.Confirmations))
	}
	if _, err := os.Stat(filepath.Join(dir, "counterexample-was-executed")); !os.IsNotExist(err) {
		t.Fatal("counterexample source was materialized after the clean baseline failed")
	}
	assertFileBytes(t, source, []byte("clean\n"))
	runs, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(runs) != "clean\n" {
		t.Fatalf("runs = %q, want exactly one clean baseline", runs)
	}
}

func TestCounterexampleExecutorBaselineRunsForVacuousConfirmation(t *testing.T) {
	dir := t.TempDir()
	script := writeExecutorScript(t, dir, "printf 'baseline' > baseline-ran\nexit 0\n")
	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, nil)

	if report.Status != executor.StatusConfirmed || !report.Vacuous || !report.Baseline.Passed {
		t.Fatalf("report = %+v, want a passing vacuous confirmation with a real baseline", report)
	}
	assertFileBytes(t, filepath.Join(dir, "baseline-ran"), []byte("baseline"))
}

func TestCounterexampleExecutorBaselineRejectsStaleArtifactBeforeRunning(t *testing.T) {
	dir := t.TempDir()
	source := writeExecutorFixture(t, dir, "subject.txt", []byte("current\n"))
	script := writeExecutorScript(t, dir, "touch verifier-ran\nexit 0\n")
	plan := executorPlan("stale-plan", source, []byte("frozen\n"), []byte("changed\n"), true)

	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})

	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "stale-artifact") {
		t.Fatalf("report = %+v, want stale-artifact blocker", report)
	}
	if _, err := os.Stat(filepath.Join(dir, "verifier-ran")); !os.IsNotExist(err) {
		t.Fatal("verifier ran even though the edit plan's frozen digest was stale")
	}
	assertFileBytes(t, source, []byte("current\n"))
}

func TestCounterexampleExecutorBaselineRestoresSourceSideEffects(t *testing.T) {
	dir := t.TempDir()
	original := []byte("clean\n\x00baseline\r\n")
	source := writeExecutorFixture(t, dir, "subject.txt", original)
	script := writeExecutorScript(t, dir, `
if grep -q '^clean' subject.txt; then
  printf 'baseline-corrupted-source\n' > subject.txt
  exit 0
fi
exit 0
`)
	plan := executorPlan("baseline-side-effect", source, original, []byte("counterexample\n"), true)

	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})

	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "baseline-mutated-source") {
		t.Fatalf("report = %+v, want baseline-mutated-source blocker", report)
	}
	if len(report.Confirmations) != 0 {
		t.Fatal("counterexample executed after the baseline changed frozen source")
	}
	assertFileBytes(t, source, original)
}

func TestCounterexampleExecutorContainmentRejectsAbsoluteOutsidePath(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "workspace")
	if err := os.Mkdir(workDir, 0o750); err != nil {
		t.Fatal(err)
	}
	original := []byte("outside\n")
	source := writeExecutorFixture(t, root, "outside.txt", original)
	assertExecutorPathBlocked(t, workDir, source, source, original)
}

func TestCounterexampleExecutorContainmentRejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "workspace")
	if err := os.Mkdir(workDir, 0o750); err != nil {
		t.Fatal(err)
	}
	original := []byte("outside\n")
	source := writeExecutorFixture(t, root, "outside.txt", original)
	assertExecutorPathBlocked(t, workDir, "../outside.txt", source, original)
}

func TestCounterexampleExecutorContainmentRejectsSymlinkParentEscape(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "workspace")
	outsideDir := filepath.Join(root, "outside")
	if err := os.Mkdir(workDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outsideDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(workDir, "linked-parent")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	original := []byte("outside\n")
	source := writeExecutorFixture(t, outsideDir, "subject.txt", original)
	assertExecutorPathBlocked(t, workDir, filepath.Join("linked-parent", "subject.txt"), source, original)
}

func TestCounterexampleExecutorVerdictExitCodeSignal(t *testing.T) {
	dir := t.TempDir()
	source := writeExecutorFixture(t, dir, "subject.txt", []byte("clean\n"))
	script := writeExecutorScript(t, dir, `
if grep -q '^clean$' subject.txt; then
  exit 0
fi
exit 17
`)
	plan := executorPlan("exit-plan", source, []byte("clean\n"), []byte("counterexample\n"), false)

	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})

	assertConfirmedExecutorReport(t, report)
	confirmation := report.Confirmations[0]
	if confirmation.ObservedTestPasses == nil || *confirmation.ObservedTestPasses {
		t.Fatalf("observed pass = %v, want false from exit 17", confirmation.ObservedTestPasses)
	}
	if confirmation.Command.ExitCode == nil || *confirmation.Command.ExitCode != 17 {
		t.Fatalf("exit evidence = %v, want 17", confirmation.Command.ExitCode)
	}
	assertFileBytes(t, source, []byte("clean\n"))
}

func TestCounterexampleExecutorVerdictModelExecutionMismatchBlocks(t *testing.T) {
	dir := t.TempDir()
	source := writeExecutorFixture(t, dir, "subject.txt", []byte("clean\n"))
	script := writeExecutorScript(t, dir, `
if [ "$RAY_EXECUTOR_DECLARED" != "present" ]; then exit 90; fi
if grep -q '^clean$' subject.txt; then exit 0; fi
exit 41
`)
	// The pre-existing proof witness expects the violating artifact to pass.
	// Execution observes failure. Under the frozen architecture this is a
	// translation/model defect, not a different proof result.
	plan := executorPlan("mismatch-plan", source, []byte("clean\n"), []byte("counterexample\n"), true)
	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		Environment: []string{"RAY_EXECUTOR_DECLARED=present"},
		PassSignal:  executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})

	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "model-execution-mismatch") {
		t.Fatalf("report = %+v, want a model/execution mismatch blocker", report)
	}
	if len(report.Confirmations) != 1 || report.Confirmations[0].Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Confirmations[0].Blockers, "model-execution-mismatch") {
		t.Fatalf("confirmations = %+v, want one blocked proof witness", report.Confirmations)
	}
	if report.Confirmations[0].ObservedTestPasses == nil || *report.Confirmations[0].ObservedTestPasses {
		t.Fatalf("observed pass = %v, want false", report.Confirmations[0].ObservedTestPasses)
	}
	assertFileBytes(t, source, []byte("clean\n"))
}

func TestCounterexampleExecutorVerdictFileRequiresFreshSignal(t *testing.T) {
	dir := t.TempDir()
	source := writeExecutorFixture(t, dir, "subject.txt", []byte("clean\n"))
	verdict := filepath.Join(dir, "reward.txt")
	// This pre-existing PASS is intentionally stale. The baseline creates a
	// fresh PASS, while the counterexample creates no verdict at all. A runner
	// that reuses either stale value would incorrectly confirm the witness.
	writeExecutorFixture(t, dir, "reward.txt", []byte("PASS\n"))
	script := writeExecutorScript(t, dir, `
if grep -q '^clean$' subject.txt; then
  printf 'PASS\n' > reward.txt
fi
exit 0
`)
	plan := executorPlan("fresh-plan", source, []byte("clean\n"), []byte("counterexample\n"), true)

	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.VerdictFileSignal("reward.txt", "PASS"),
	}, []semanticir.EditPlan{plan})

	if report.Status != executor.StatusProofBlocked {
		t.Fatalf("status = %q, want blocked for missing fresh verdict", report.Status)
	}
	if len(report.Confirmations) != 1 {
		t.Fatalf("confirmations = %d, want 1", len(report.Confirmations))
	}
	confirmation := report.Confirmations[0]
	if confirmation.Status != executor.StatusProofBlocked || confirmation.Command.Signal.FreshVerdict {
		t.Fatalf("confirmation = %+v, want missing-fresh-verdict blocker", confirmation)
	}
	if !hasExecutorBlocker(confirmation.Blockers, "stale-or-missing-verdict") {
		t.Fatalf("blockers = %+v, want stale-or-missing-verdict", confirmation.Blockers)
	}
	assertFileBytes(t, source, []byte("clean\n"))
	// Executor cleanup restores the verdict artifact that existed before the
	// whole operation, but never treats it as evidence for either run.
	assertFileBytes(t, verdict, []byte("PASS\n"))
}

func TestCounterexampleExecutorVerdictFileAcceptsFreshValue(t *testing.T) {
	dir := t.TempDir()
	source := writeExecutorFixture(t, dir, "subject.txt", []byte("clean\n"))
	script := writeExecutorScript(t, dir, `
printf 'PASS\n' > reward.txt
exit 99
`)
	plan := executorPlan("file-plan", source, []byte("clean\n"), []byte("counterexample\n"), true)

	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.VerdictFileSignal("reward.txt", "PASS"),
	}, []semanticir.EditPlan{plan})

	assertConfirmedExecutorReport(t, report)
	confirmation := report.Confirmations[0]
	if !confirmation.Command.Signal.FreshVerdict || !confirmation.Command.Passed {
		t.Fatalf("command evidence = %+v, want fresh PASS", confirmation.Command)
	}
	if _, err := os.Stat(filepath.Join(dir, "reward.txt")); !os.IsNotExist(err) {
		t.Fatal("generated verdict file was not cleaned up")
	}
	assertFileBytes(t, source, []byte("clean\n"))
}

func TestCounterexampleExecutorRestoreAfterSuccess(t *testing.T) {
	testExecutorRestoration(t, "success", `
if grep -q '^clean$' subject.txt; then exit 0; fi
printf 'observed' > execution-observed
exit 0
`, true, nil)
}

func TestCounterexampleExecutorRestoreAfterFailure(t *testing.T) {
	testExecutorRestoration(t, "failure", `
if grep -q '^clean$' subject.txt; then exit 0; fi
printf 'observed' > execution-observed
exit 31
`, false, nil)
}

func TestCounterexampleExecutorRestoreAfterTimeout(t *testing.T) {
	testExecutorRestoration(t, "timeout", `
if grep -q '^clean$' subject.txt; then exit 0; fi
printf 'observed' > execution-observed
sleep 10
exit 0
`, true, func(task *executor.TaskEnvironment, _ context.CancelFunc) {
		// Leave enough headroom for the mandatory clean baseline under race or
		// stress load while still forcing the witness's long-lived child down
		// the timeout/process-group cleanup path.
		task.Timeout = 200 * time.Millisecond
	})
}

func TestCounterexampleExecutorRestoreAfterInterruption(t *testing.T) {
	dir := t.TempDir()
	original := []byte("clean\n\x00exact\r\n")
	source := writeExecutorFixture(t, dir, "subject.txt", original)
	script := writeExecutorScript(t, dir, `
if grep -q '^clean' subject.txt; then exit 0; fi
printf 'started' > execution-observed
sleep 10
exit 0
`)
	plan := executorPlan("interrupt-plan", source, original, []byte("counterexample\n"), true)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan executor.Report, 1)
	go func() {
		done <- executor.Confirm(ctx, executor.TaskEnvironment{
			Command: []string{"sh", script}, WorkDir: dir, Timeout: 20 * time.Second,
			PassSignal: executor.ExitCodeSignal(0),
		}, []semanticir.EditPlan{plan})
	}()
	waitForExecutorFile(t, filepath.Join(dir, "execution-observed"), time.Second)
	cancel()

	var report executor.Report
	select {
	case report = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not return promptly after context cancellation")
	}
	if report.Status != executor.StatusProofBlocked || len(report.Confirmations) != 1 {
		t.Fatalf("report = %+v, want one blocked confirmation", report)
	}
	confirmation := report.Confirmations[0]
	if !confirmation.Command.Interrupted || len(confirmation.Materializations) != 1 || !confirmation.Materializations[0].Restored {
		t.Fatalf("confirmation = %+v, want interrupted run with restoration", confirmation)
	}
	assertFileBytes(t, source, original)
}

func TestCounterexampleExecutorRestoreAppliesMultipleByteRangesExactly(t *testing.T) {
	dir := executorCanonicalPath(t, t.TempDir())
	original := []byte("alpha beta gamma\n")
	source := writeExecutorFixture(t, dir, "subject.txt", original)
	script := writeExecutorScript(t, dir, `
if grep -q '^alpha beta gamma$' subject.txt; then exit 0; fi
grep -q '^A beta G$' subject.txt
`)
	artifact := semanticir.ArtifactRef{
		ID: "code", Kind: semanticir.ArtifactCode, Path: source, Digest: executorTestDigest(original),
	}
	plan := semanticir.EditPlan{
		ID: "range-plan", WitnessID: "w-range", Artifact: artifact,
		// Deliberately supplied out of order; byte coordinates are relative to
		// the frozen artifact and must not shift as replacements change length.
		Edits: []semanticir.ByteRangeReplacement{
			{StartByte: 11, EndByte: 16, ExpectedBytes: []byte("gamma"), Replacement: []byte("G")},
			{StartByte: 0, EndByte: 5, ExpectedBytes: []byte("alpha"), Replacement: []byte("A")},
		},
		Expected: semanticir.ExpectedSemantics{OperationID: "operation", OutcomeIDs: []string{"outcome"}, TestPasses: true},
	}
	executorBindPlanProvenance(&plan)

	workspaceDigest := executorWorkspaceDigest(t, dir)
	report := executor.ConfirmIsolated(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkspaceRoot: dir, WorkspaceSHA256: workspaceDigest, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})

	assertConfirmedExecutorReport(t, report)
	materialization := report.Confirmations[0].Materializations[0]
	if materialization.MaterializedSize != len("A beta G\n") || len(materialization.Edits) != 2 {
		t.Fatalf("materialization evidence = %+v", materialization)
	}
	if err := executor.ValidateEditConfirmation(report.Confirmations[0]); err != nil {
		t.Fatalf("unsorted exact edit plan produced invalid certificate evidence: %v", err)
	}
	assertFileBytes(t, source, original)
}

func TestCounterexampleExecutorRestoreRejectsNoOpAndOverlappingPlans(t *testing.T) {
	dir := t.TempDir()
	original := []byte("abcdef")
	source := writeExecutorFixture(t, dir, "subject.txt", original)
	script := writeExecutorScript(t, dir, "touch verifier-ran\nexit 0\n")
	artifact := semanticir.ArtifactRef{ID: "code", Kind: semanticir.ArtifactCode, Path: source, Digest: executorTestDigest(original)}

	cases := map[string]struct {
		edits   []semanticir.ByteRangeReplacement
		blocker string
	}{
		"no-op": {
			edits:   []semanticir.ByteRangeReplacement{{StartByte: 1, EndByte: 3, ExpectedBytes: []byte("bc"), Replacement: []byte("bc")}},
			blocker: "no-op-edit-plan",
		},
		"empty-insertion": {
			edits:   []semanticir.ByteRangeReplacement{{StartByte: 2, EndByte: 2, ExpectedBytes: []byte{}, Replacement: []byte{}}},
			blocker: "no-op-edit-plan",
		},
		"overlap": {
			edits: []semanticir.ByteRangeReplacement{
				{StartByte: 1, EndByte: 4, ExpectedBytes: []byte("bcd"), Replacement: []byte("X")},
				{StartByte: 3, EndByte: 5, ExpectedBytes: []byte("de"), Replacement: []byte("Y")},
			},
			blocker: "overlapping-edits",
		},
		"same-offset-insertions": {
			edits: []semanticir.ByteRangeReplacement{
				{StartByte: 2, EndByte: 2, ExpectedBytes: []byte{}, Replacement: []byte("X")},
				{StartByte: 2, EndByte: 2, ExpectedBytes: []byte{}, Replacement: []byte("Y")},
			},
			blocker: "overlapping-edits",
		},
	}
	for name, testCase := range cases {
		edits := testCase.edits
		blocker := testCase.blocker
		t.Run(name, func(t *testing.T) {
			plan := semanticir.EditPlan{
				ID: name, WitnessID: "w-" + name, Artifact: artifact, Edits: edits,
				Expected: semanticir.ExpectedSemantics{OperationID: "operation", OutcomeIDs: []string{"outcome"}, TestPasses: true},
			}
			executorBindPlanProvenance(&plan)
			report := executor.Confirm(context.Background(), executor.TaskEnvironment{
				Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
				PassSignal: executor.ExitCodeSignal(0),
			}, []semanticir.EditPlan{plan})
			if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, blocker) {
				t.Fatalf("report = %+v, want %s blocker", report, blocker)
			}
			assertFileBytes(t, source, original)
		})
	}
	if _, err := os.Stat(filepath.Join(dir, "verifier-ran")); !os.IsNotExist(err) {
		t.Fatal("verifier ran for an invalid edit plan")
	}
}

func TestCounterexampleExecutorRestoreGroupsMultiArtifactWitnessAtomically(t *testing.T) {
	dir := t.TempDir()
	firstOriginal := []byte("first-clean\n")
	secondOriginal := []byte("second-clean\n")
	first := writeExecutorFixture(t, dir, "first.txt", firstOriginal)
	second := writeExecutorFixture(t, dir, "second.txt", secondOriginal)
	script := writeExecutorScript(t, dir, `
printf 'run\n' >> runs.log
if grep -q '^first-clean$' first.txt && grep -q '^second-clean$' second.txt; then
  exit 0
fi
if grep -q '^first-counterexample$' first.txt && grep -q '^second-counterexample$' second.txt; then
  exit 0
fi
exit 72
`)
	firstPlan := executorPlan("multi-first", first, firstOriginal, []byte("first-counterexample\n"), true)
	secondPlan := executorPlan("multi-second", second, secondOriginal, []byte("second-counterexample\n"), true)
	secondPlan.WitnessID = firstPlan.WitnessID
	secondPlan.Artifact.ID = "code-artifact-second"
	executorBindPlanProvenance(&secondPlan)

	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{firstPlan, secondPlan})

	assertConfirmedExecutorReport(t, report)
	confirmation := report.Confirmations[0]
	if len(confirmation.PlanIDs) != 2 || len(confirmation.Materializations) != 2 {
		t.Fatalf("confirmation = %+v, want two plans materialized in one witness run", confirmation)
	}
	for _, materialization := range confirmation.Materializations {
		if !materialization.Applied || !materialization.Restored {
			t.Fatalf("materialization = %+v, want applied and restored", materialization)
		}
	}
	assertFileBytes(t, first, firstOriginal)
	assertFileBytes(t, second, secondOriginal)
	assertFileBytes(t, filepath.Join(dir, "runs.log"), []byte("run\nrun\n"))
}

func TestCounterexampleExecutorProbeConfirmsReferenceWitness(t *testing.T) {
	fixture := newExecutorProbeFixture(t, nil)
	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})

	assertConfirmedExecutorReport(t, report)
	confirmation := report.Confirmations[0]
	if confirmation.Mode != executor.ConfirmationModeProbe || confirmation.Probe == nil || len(confirmation.Materializations) != 0 {
		t.Fatalf("confirmation = %+v, want probe-only evidence", confirmation)
	}
	probe := confirmation.Probe
	if !probe.FreshObservation || !probe.SemanticsMatch || probe.Observed == nil || !probe.HarnessRemoved || !probe.IsolatedWorkspaceRemoved || !probe.OriginalWorkspaceIntact {
		t.Fatalf("probe evidence = %+v, want fresh matching semantics and complete cleanup", probe)
	}
	if confirmation.Command.Stdout != "probe stdout" || confirmation.Command.Stderr != "probe stderr" || confirmation.Command.SignalValueSHA256 != executorTestDigest([]byte("0")) {
		t.Fatalf("command evidence = %+v, want complete output and signal evidence", confirmation.Command)
	}
	if report.BaselineIsolation == nil || !report.BaselineIsolation.IsolatedRemoved || !report.BaselineIsolation.OriginalIntact {
		t.Fatalf("probe baseline did not retain fresh isolated cleanup evidence: %+v", report.BaselineIsolation)
	}
	if err := executor.ValidateProbeConfirmation(confirmation); err != nil {
		t.Fatalf("certificate-facing probe evidence rejected: %v", err)
	}
	if fixture.plan.Witness.TestPasses {
		t.Fatal("fixture must demonstrate that reference confirmation is independent of TestPasses")
	}
	assertProbeWorkspaceUnchanged(t, fixture)
}

func TestCounterexampleExecutorProbeRunsTypedCompileOutput(t *testing.T) {
	fixture := newExecutorProbeFixture(t, nil)
	payload := fixture.plan.Steps[0].Argv[2]
	generated := ".ray/probes/generated-probe"
	tool := fixture.plan.Tools[0]
	fixture.plan.Steps = []executor.ProbeStep{
		{
			ID: "compile", Kind: executor.ProbeStepCompile, Tool: &tool,
			Argv:    []string{tool.Path, "-c", "cp .ray/probes/probe.sh .ray/probes/generated-probe && chmod 700 .ray/probes/generated-probe"},
			WorkDir: ".", Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0), Outputs: []string{generated},
		},
		{
			ID: "run", Kind: executor.ProbeStepRun, GeneratedExecutable: generated,
			Argv: []string{generated, payload}, WorkDir: ".", Timeout: time.Second,
			PassSignal: executor.ExitCodeSignal(0), ObservationPath: ".ray/probes/probe-observation.json",
		},
	}
	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})

	assertConfirmedExecutorReport(t, report)
	probe := report.Confirmations[0].Probe
	if probe == nil || len(probe.Steps) != 2 || len(probe.Steps[0].Outputs) != 1 {
		t.Fatalf("ordered step evidence missing: %+v", probe)
	}
	output := probe.Steps[0].Outputs[0]
	if output.Path != generated || output.ProducerStepID != "compile" || !output.Fresh || !output.VerifiedBeforeRun || !output.VerifiedAfterRun || output.Mode&0o111 == 0 || !semanticir.ValidDigest(output.SHA256) {
		t.Fatalf("generated executable evidence incomplete: %+v", output)
	}
	if !reflect.DeepEqual(report.Confirmations[0].Command, probe.Steps[1].Command) {
		t.Fatal("confirmation command does not mirror the final ordered run step")
	}
	if err := executor.ValidateProbeConfirmation(report.Confirmations[0]); err != nil {
		t.Fatalf("ordered probe confirmation rejected: %v", err)
	}
	tampered := cloneExecutorConfirmation(t, report.Confirmations[0])
	tampered.Probe.Steps[0].Outputs[0].SHA256 = executorTestDigest([]byte("different compiler output"))
	if err := executor.ValidateProbeConfirmation(tampered); err == nil {
		t.Fatal("tampered generated compiler output passed validation")
	}
	assertProbeWorkspaceUnchanged(t, fixture)
}

func TestCounterexampleExecutorProbeRejectsNonFreshCompileOutput(t *testing.T) {
	fixture := newExecutorProbeFixture(t, nil)
	payload := fixture.plan.Steps[0].Argv[2]
	generated := ".ray/probes/generated-probe"
	tool := fixture.plan.Tools[0]
	if err := os.MkdirAll(filepath.Dir(filepath.Join(fixture.root, filepath.FromSlash(generated))), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, filepath.FromSlash(generated)), []byte("preexisting"), 0o700); err != nil {
		t.Fatal(err)
	}
	fixture.plan.Workspace.TreeSHA256 = executorWorkspaceDigest(t, fixture.root)
	fixture.workspaceDigest = fixture.plan.Workspace.TreeSHA256
	fixture.plan.Steps = []executor.ProbeStep{
		{ID: "compile", Kind: executor.ProbeStepCompile, Tool: &tool, Argv: []string{tool.Path, "-c", "exit 0"}, WorkDir: ".", Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0), Outputs: []string{generated}},
		{ID: "run", Kind: executor.ProbeStepRun, GeneratedExecutable: generated, Argv: []string{generated, payload}, WorkDir: ".", Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0), ObservationPath: ".ray/probes/probe-observation.json"},
	}
	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})

	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "probe-output-not-fresh") {
		t.Fatalf("report = %+v, want non-fresh compile-output blocker", report)
	}
	assertProbeWorkspaceUnchanged(t, fixture)
}

func TestCounterexampleExecutorProbeRejectsChangedGeneratedExecutable(t *testing.T) {
	fixture := newExecutorProbeFixture(t, nil)
	payload := fixture.plan.Steps[0].Argv[2]
	generated := ".ray/probes/generated-probe"
	tool := fixture.plan.Tools[0]
	fixture.plan.Harness.Bytes = append(append([]byte(nil), fixture.plan.Harness.Bytes...), []byte("printf '# changed\\n' >> \"$0\"\n")...)
	fixture.plan.Harness.SHA256 = executorTestDigest(fixture.plan.Harness.Bytes)
	fixture.plan.Steps = []executor.ProbeStep{
		{ID: "compile", Kind: executor.ProbeStepCompile, Tool: &tool, Argv: []string{tool.Path, "-c", "cp .ray/probes/probe.sh .ray/probes/generated-probe && chmod 700 .ray/probes/generated-probe"}, WorkDir: ".", Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0), Outputs: []string{generated}},
		{ID: "run", Kind: executor.ProbeStepRun, GeneratedExecutable: generated, Argv: []string{generated, payload}, WorkDir: ".", Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0), ObservationPath: ".ray/probes/probe-observation.json"},
	}
	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})

	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "probe-generated-executable-changed") {
		t.Fatalf("report = %+v, want changed generated-executable blocker", report)
	}
	assertProbeWorkspaceUnchanged(t, fixture)
}

func TestCounterexampleExecutorProbeRejectsStaleBindingsBeforeBaseline(t *testing.T) {
	t.Run("workspace", func(t *testing.T) {
		fixture := newExecutorProbeFixture(t, nil)
		if err := os.WriteFile(fixture.source, []byte("stale workspace\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
		if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "stale-probe-workspace") || !report.Baseline.StartedAt.IsZero() {
			t.Fatalf("report = %+v, want pre-baseline stale workspace blocker", report)
		}
	})

	t.Run("source", func(t *testing.T) {
		fixture := newExecutorProbeFixture(t, nil)
		if err := os.WriteFile(fixture.source, []byte("stale source\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		fixture.plan.Workspace.TreeSHA256 = executorWorkspaceDigest(t, fixture.root)
		report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
		if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "stale-probe-source") || !report.Baseline.StartedAt.IsZero() {
			t.Fatalf("report = %+v, want pre-baseline stale source blocker", report)
		}
	})

	t.Run("tool", func(t *testing.T) {
		fixture := newExecutorProbeFixture(t, nil)
		fixture.plan.Tools[0].Digest = executorTestDigest([]byte("different executable"))
		report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
		if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "stale-probe-tool") || !report.Baseline.StartedAt.IsZero() {
			t.Fatalf("report = %+v, want pre-baseline stale tool blocker", report)
		}
	})
}

func TestCounterexampleExecutorProbeRejectsPathEscapesBeforeBaseline(t *testing.T) {
	t.Run("harness-parent-traversal", func(t *testing.T) {
		fixture := newExecutorProbeFixture(t, nil)
		fixture.plan.Harness.Path = "../probe.sh"
		report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
		if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "probe-harness-path") || !report.Baseline.StartedAt.IsZero() {
			t.Fatalf("report = %+v, want pre-baseline harness escape blocker", report)
		}
	})

	t.Run("absolute-source", func(t *testing.T) {
		fixture := newExecutorProbeFixture(t, nil)
		outside := writeExecutorFixture(t, t.TempDir(), "outside.txt", []byte("solution\n"))
		fixture.plan.SourceArtifacts[0].Path = outside
		report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
		if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "probe-source-path") || !report.Baseline.StartedAt.IsZero() {
			t.Fatalf("report = %+v, want pre-baseline absolute source blocker", report)
		}
	})

	t.Run("symlink-parent", func(t *testing.T) {
		fixture := newExecutorProbeFixture(t, nil)
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(fixture.root, "outside-link")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		fixture.plan.Workspace.TreeSHA256 = executorWorkspaceDigest(t, fixture.root)
		fixture.plan.Harness.Path = "outside-link/probe.sh"
		report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
		if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "probe-harness-path") || !report.Baseline.StartedAt.IsZero() {
			t.Fatalf("report = %+v, want pre-baseline symlink-parent blocker", report)
		}
	})
}

func TestCounterexampleExecutorProbeModelExecutionMismatchBlocks(t *testing.T) {
	fixture := newExecutorProbeFixture(t, nil)
	observed := cloneProbeObservation(t, executor.ProbeObservation{Traces: []semanticir.RawOutcomeTrace{fixture.plan.ExpectedSemantics.RuntimeOutcomes[0].RawOutcome}})
	observed.Traces[0].Value.String = "alternate"
	setExecutorProbeObservation(t, &fixture.plan, observed)

	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "model-execution-mismatch") || len(report.Confirmations) != 1 {
		t.Fatalf("report = %+v, want model/execution mismatch blocker", report)
	}
	confirmation := report.Confirmations[0]
	if confirmation.Status != executor.StatusProofBlocked || !hasExecutorBlocker(confirmation.Blockers, "model-execution-mismatch") || !confirmation.Command.Passed || confirmation.Probe == nil || !confirmation.Probe.FreshObservation || confirmation.Probe.SemanticsMatch {
		t.Fatalf("confirmation = %+v, want passing command with mismatched observed semantics", confirmation)
	}
	assertProbeWorkspaceUnchanged(t, fixture)
}

func TestCounterexampleExecutorProbeRejectsProcessSuppliedSemanticFields(t *testing.T) {
	for _, field := range []string{"id", "provenance"} {
		t.Run(field, func(t *testing.T) {
			fixture := newExecutorProbeFixture(t, nil)
			payload := fixture.plan.Steps[len(fixture.plan.Steps)-1].Argv[2]
			payload = strings.Replace(payload, `{"traces":[{`, `{"traces":[{"`+field+`":"forged",`, 1)
			fixture.plan.Steps[len(fixture.plan.Steps)-1].Argv[2] = payload

			report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
			if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "probe-observation-invalid") {
				t.Fatalf("report = %+v, want forged semantic-field blocker", report)
			}
			if len(report.Confirmations) != 1 || report.Confirmations[0].Probe == nil || report.Confirmations[0].Probe.FreshObservation || report.Confirmations[0].Probe.Observed != nil {
				t.Fatalf("forged process semantics became evidence: %+v", report.Confirmations)
			}
			assertProbeWorkspaceUnchanged(t, fixture)
		})
	}
}

func TestCounterexampleExecutorProbeRejectsCompileStepObservation(t *testing.T) {
	fixture := newExecutorProbeFixture(t, nil)
	payload := fixture.plan.Steps[0].Argv[2]
	tool := fixture.plan.Tools[0]
	fixture.plan.Steps = []executor.ProbeStep{
		{
			ID: "compile", Kind: executor.ProbeStepCompile, Tool: &tool,
			Argv:    []string{tool.Path, "-c", "printf '%s' \"$1\" > .ray/probes/probe-observation.json; cp .ray/probes/probe.sh .ray/probes/generated-probe; chmod 700 .ray/probes/generated-probe", "compile", payload},
			WorkDir: ".", Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0), Outputs: []string{".ray/probes/generated-probe"},
		},
		{
			ID: "run", Kind: executor.ProbeStepRun, Tool: &tool,
			Argv: []string{tool.Path, "-c", "exit 0"}, WorkDir: ".", Timeout: time.Second,
			PassSignal: executor.ExitCodeSignal(0), ObservationPath: ".ray/probes/probe-observation.json",
		},
	}

	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "probe-observation-invalid") {
		t.Fatalf("report = %+v, want compile-produced observation blocker", report)
	}
	if len(report.Confirmations) != 1 || report.Confirmations[0].Probe == nil || report.Confirmations[0].Probe.FreshObservation {
		t.Fatalf("compile-produced observation became run evidence: %+v", report.Confirmations)
	}
	assertProbeWorkspaceUnchanged(t, fixture)
}

func TestCounterexampleExecutorProbeTimeoutKillsChildAndCleansWorkspace(t *testing.T) {
	fixture := newExecutorProbeFixture(t, nil)
	fixture.plan.Steps[0].Timeout = 80 * time.Millisecond
	fixture.plan.Harness.Bytes = []byte("#!/bin/sh\nset -eu\nprintf '%s' \"$1\" > .ray/probes/probe-observation.json\nsleep 10 &\nprintf 'child:%s\\n' \"$!\"\nwait\n")
	fixture.plan.Harness.SHA256 = executorTestDigest(fixture.plan.Harness.Bytes)

	started := time.Now()
	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
	if time.Since(started) > 2*time.Second {
		t.Fatal("timed-out probe did not terminate its process group promptly")
	}
	if report.Status != executor.StatusProofBlocked || len(report.Confirmations) != 1 || !hasExecutorBlocker(report.Blockers, "timeout") {
		t.Fatalf("report = %+v, want timeout blocker", report)
	}
	confirmation := report.Confirmations[0]
	if !confirmation.Command.TimedOut || confirmation.Probe == nil || !confirmation.Probe.HarnessRemoved || !confirmation.Probe.IsolatedWorkspaceRemoved || !confirmation.Probe.OriginalWorkspaceIntact {
		t.Fatalf("confirmation = %+v, want timeout with complete cleanup", confirmation)
	}
	assertProbeWorkspaceUnchanged(t, fixture)
}

func TestCounterexampleExecutorProbeRestoresPreexistingHarnessExactly(t *testing.T) {
	originalHarness := []byte("pre-existing harness\n\x00exact\r\n")
	fixture := newExecutorProbeFixture(t, originalHarness)
	beforeInfo, err := os.Stat(filepath.Join(fixture.root, fixture.plan.Harness.Path))
	if err != nil {
		t.Fatal(err)
	}
	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})

	assertConfirmedExecutorReport(t, report)
	probe := report.Confirmations[0].Probe
	if probe == nil || !probe.HarnessPreviouslyExisted || !probe.HarnessRestored || probe.HarnessRemoved {
		t.Fatalf("probe = %+v, want exact pre-existing harness restoration", probe)
	}
	assertFileBytes(t, filepath.Join(fixture.root, fixture.plan.Harness.Path), originalHarness)
	afterInfo, err := os.Stat(filepath.Join(fixture.root, fixture.plan.Harness.Path))
	if err != nil || afterInfo.Mode().Perm() != beforeInfo.Mode().Perm() {
		t.Fatalf("restored harness mode = %v, want %v (err=%v)", afterInfo.Mode().Perm(), beforeInfo.Mode().Perm(), err)
	}
	assertProbeWorkspaceUnchanged(t, fixture)
}

func TestCounterexampleExecutorProbeValidatorRejectsTamperedEvidence(t *testing.T) {
	fixture := newExecutorProbeFixture(t, nil)
	report := executor.ConfirmProbes(context.Background(), fixture.baseline, []executor.ProbePlan{fixture.plan})
	assertConfirmedExecutorReport(t, report)

	cases := map[string]func(*executor.Confirmation){
		"observed-semantics": func(confirmation *executor.Confirmation) {
			confirmation.Probe.Observed.Traces[0].Value.String = "tampered"
		},
		"source-binding": func(confirmation *executor.Confirmation) {
			confirmation.Probe.SourceBindings[0].Verified = false
		},
		"tool-postcheck": func(confirmation *executor.Confirmation) {
			confirmation.Probe.ToolsVerifiedAfterRun = false
		},
		"stdout-digest": func(confirmation *executor.Confirmation) {
			confirmation.Command.StdoutSHA256 = executorTestDigest([]byte("tampered"))
		},
		"signal-digest": func(confirmation *executor.Confirmation) {
			confirmation.Command.SignalValueSHA256 = executorTestDigest([]byte("99"))
		},
		"cleanup": func(confirmation *executor.Confirmation) {
			confirmation.Probe.IsolatedWorkspaceRemoved = false
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			confirmation := cloneExecutorConfirmation(t, report.Confirmations[0])
			mutate(&confirmation)
			if err := executor.ValidateProbeConfirmation(confirmation); err == nil {
				t.Fatalf("tampered confirmation passed validation: %+v", confirmation)
			}
		})
	}
}

func TestCounterexampleExecutorWorkspaceRootAllowsNestedCommandWorkDir(t *testing.T) {
	root := executorCanonicalPath(t, t.TempDir())
	workDir := filepath.Join(root, "build")
	if err := os.Mkdir(workDir, 0o750); err != nil {
		t.Fatal(err)
	}
	original := []byte("clean\n")
	source := writeExecutorFixture(t, root, "subject.txt", original)
	script := writeExecutorScript(t, root, "grep -q '^clean$' ../subject.txt || grep -q '^changed$' ../subject.txt\n")
	plan := executorPlan("nested-workdir", source, original, []byte("changed\n"), true)
	plan.Artifact.Path = "subject.txt"

	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkspaceRoot: root, WorkDir: workDir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})
	assertConfirmedExecutorReport(t, report)
	if report.Baseline.WorkDir != workDir || report.Confirmations[0].Command.WorkDir != workDir {
		t.Fatalf("command workdirs = %q/%q, want exact nested cwd %q", report.Baseline.WorkDir, report.Confirmations[0].Command.WorkDir, workDir)
	}
	assertFileBytes(t, source, original)
}

func TestCounterexampleExecutorWorkspaceRootResolvesRelativeVerdictFromRoot(t *testing.T) {
	root := executorCanonicalPath(t, t.TempDir())
	workDir := filepath.Join(root, "build")
	if err := os.Mkdir(workDir, 0o750); err != nil {
		t.Fatal(err)
	}
	original := []byte("clean\n")
	source := writeExecutorFixture(t, root, "subject.txt", original)
	script := writeExecutorScript(t, root, "printf 'PASS\\n' > ../reward.txt\n")
	plan := executorPlan("nested-verdict", source, original, []byte("changed\n"), true)
	plan.Artifact.Path = "subject.txt"

	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkspaceRoot: root, WorkDir: workDir, Timeout: time.Second,
		PassSignal: executor.VerdictFileSignal("reward.txt", "PASS"),
	}, []semanticir.EditPlan{plan})
	assertConfirmedExecutorReport(t, report)
	if report.Baseline.Signal.VerdictPath != filepath.Join(root, "reward.txt") || report.Confirmations[0].Command.Signal.VerdictPath != filepath.Join(root, "reward.txt") {
		t.Fatalf("verdict paths = %q/%q, want workspace-root-relative path", report.Baseline.Signal.VerdictPath, report.Confirmations[0].Command.Signal.VerdictPath)
	}
	if _, err := os.Stat(filepath.Join(root, "reward.txt")); !os.IsNotExist(err) {
		t.Fatal("generated root-relative verdict was not removed")
	}
	assertFileBytes(t, source, original)
}

func TestCounterexampleExecutorWorkspaceRootRejectsEscapedWorkDirAndVerdict(t *testing.T) {
	root := executorCanonicalPath(t, t.TempDir())
	outside := executorCanonicalPath(t, t.TempDir())
	script := writeExecutorScript(t, root, "touch verifier-ran\nexit 0\n")

	t.Run("workdir", func(t *testing.T) {
		report := executor.Confirm(context.Background(), executor.TaskEnvironment{
			Command: []string{"sh", script}, WorkspaceRoot: root, WorkDir: outside, Timeout: time.Second,
			PassSignal: executor.ExitCodeSignal(0),
		}, nil)
		if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "workdir-outside-workspace") || !report.Baseline.StartedAt.IsZero() {
			t.Fatalf("report = %+v, want pre-execution escaped-workdir blocker", report)
		}
	})

	t.Run("absolute-verdict", func(t *testing.T) {
		report := executor.Confirm(context.Background(), executor.TaskEnvironment{
			Command: []string{"sh", script}, WorkspaceRoot: root, WorkDir: root, Timeout: time.Second,
			PassSignal: executor.VerdictFileSignal(filepath.Join(outside, "reward.txt"), "PASS"),
		}, nil)
		if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "verdict-outside-workspace") || !report.Baseline.StartedAt.IsZero() {
			t.Fatalf("report = %+v, want pre-execution escaped-verdict blocker", report)
		}
	})

	if _, err := os.Stat(filepath.Join(root, "verifier-ran")); !os.IsNotExist(err) {
		t.Fatal("verifier ran despite escaped workspace bindings")
	}
}

func TestCounterexampleExecutorIsolatedDoesNotShareVerifierSideEffects(t *testing.T) {
	dir := executorCanonicalPath(t, t.TempDir())
	original := []byte("clean\n")
	source := writeExecutorFixture(t, dir, "subject.txt", original)
	script := writeExecutorScript(t, dir, "test ! -e verifier-state || exit 91\ntouch verifier-state\ngrep -Eq '^(clean|changed)$' subject.txt\n")
	digest := executorWorkspaceDigest(t, dir)
	plan := executorPlan("isolated-side-effect", source, original, []byte("changed\n"), true)
	report := executor.ConfirmIsolated(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkspaceRoot: dir, WorkspaceSHA256: digest,
		WorkDir: dir, Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})

	assertConfirmedExecutorReport(t, report)
	if err := executor.ValidateEditConfirmation(report.Confirmations[0]); err != nil {
		t.Fatalf("certificate-facing isolated edit confirmation rejected: %v", err)
	}
	tampered := cloneExecutorConfirmation(t, report.Confirmations[0])
	tampered.Plans[0].Edits[0].Replacement = []byte("detached replacement\n")
	if err := executor.ValidateEditConfirmation(tampered); err == nil {
		t.Fatal("tampered full edit plan passed confirmation validation")
	}
	if report.BaselineIsolation == nil || report.Confirmations[0].Isolation == nil {
		t.Fatalf("isolation evidence missing: %+v", report)
	}
	baselineIsolation, candidateIsolation := report.BaselineIsolation, report.Confirmations[0].Isolation
	for _, evidence := range []*executor.IsolationEvidence{baselineIsolation, candidateIsolation} {
		if evidence.ExpectedSHA256 != digest || evidence.OriginalBeforeSHA256 != digest || evidence.CopyBeforeSHA256 != digest ||
			evidence.OriginalAfterSHA256 != digest || evidence.CopyAfterSHA256 == "" || !evidence.IsolatedRemoved || !evidence.OriginalIntact || evidence.Error != "" {
			t.Fatalf("incomplete isolated workspace evidence: %+v", evidence)
		}
		if _, err := os.Stat(filepath.Dir(evidence.IsolatedRoot)); !os.IsNotExist(err) {
			t.Fatalf("isolated workspace parent still exists: %s (%v)", filepath.Dir(evidence.IsolatedRoot), err)
		}
	}
	if baselineIsolation.IsolatedRoot == candidateIsolation.IsolatedRoot {
		t.Fatal("baseline and candidate shared an isolated workspace")
	}
	if _, err := os.Stat(filepath.Join(dir, "verifier-state")); !os.IsNotExist(err) {
		t.Fatal("verifier side effect escaped into the frozen workspace")
	}
	if got := executorWorkspaceDigest(t, dir); got != digest {
		t.Fatalf("frozen workspace digest = %s, want %s", got, digest)
	}
}

func TestCounterexampleExecutorIsolatedSeparatesWitnessGroups(t *testing.T) {
	dir := executorCanonicalPath(t, t.TempDir())
	oneBody, twoBody := []byte("one\n"), []byte("two\n")
	one := writeExecutorFixture(t, dir, "one.txt", oneBody)
	two := writeExecutorFixture(t, dir, "two.txt", twoBody)
	script := writeExecutorScript(t, dir, "test ! -e verifier-state || exit 92\ntouch verifier-state\nexit 0\n")
	digest := executorWorkspaceDigest(t, dir)
	plans := []semanticir.EditPlan{
		executorPlan("isolated-one", one, oneBody, []byte("changed-one\n"), true),
		executorPlan("isolated-two", two, twoBody, []byte("changed-two\n"), true),
	}
	report := executor.ConfirmIsolated(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkspaceRoot: dir, WorkspaceSHA256: digest,
		WorkDir: dir, Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0),
	}, plans)

	if report.Status != executor.StatusConfirmed || len(report.Confirmations) != 2 {
		t.Fatalf("report = %+v, want two isolated confirmations", report)
	}
	seen := map[string]bool{report.BaselineIsolation.IsolatedRoot: true}
	for _, confirmation := range report.Confirmations {
		if confirmation.Isolation == nil || seen[confirmation.Isolation.IsolatedRoot] || !confirmation.Isolation.IsolatedRemoved || !confirmation.Isolation.OriginalIntact {
			t.Fatalf("witness did not receive a distinct cleaned workspace: %+v", confirmation)
		}
		seen[confirmation.Isolation.IsolatedRoot] = true
	}
	if _, err := os.Stat(filepath.Join(dir, "verifier-state")); !os.IsNotExist(err) {
		t.Fatal("isolated witness side effect escaped into the frozen workspace")
	}
}

func TestCounterexampleExecutorIsolatedRejectsStaleWorkspaceBeforeBaseline(t *testing.T) {
	dir := executorCanonicalPath(t, t.TempDir())
	original := []byte("clean\n")
	source := writeExecutorFixture(t, dir, "subject.txt", original)
	script := writeExecutorScript(t, dir, "touch verifier-ran\nexit 0\n")
	digest := executorWorkspaceDigest(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "late-file"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := executor.ConfirmIsolated(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkspaceRoot: dir, WorkspaceSHA256: digest,
		WorkDir: dir, Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{executorPlan("isolated-stale", source, original, []byte("changed\n"), true)})

	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "stale-workspace") || !report.Baseline.StartedAt.IsZero() {
		t.Fatalf("report = %+v, want stale workspace blocker before baseline", report)
	}
	if _, err := os.Stat(filepath.Join(dir, "verifier-ran")); !os.IsNotExist(err) {
		t.Fatal("baseline ran against a stale isolated workspace binding")
	}
}

func TestCounterexampleExecutorIsolatedTimeoutCleansCopyAndPreservesOriginal(t *testing.T) {
	dir := executorCanonicalPath(t, t.TempDir())
	original := []byte("clean\n")
	source := writeExecutorFixture(t, dir, "subject.txt", original)
	script := writeExecutorScript(t, dir, "if grep -q '^clean$' subject.txt; then exit 0; fi\n(sleep 10; touch child-survived) &\nsleep 10\n")
	digest := executorWorkspaceDigest(t, dir)
	report := executor.ConfirmIsolated(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkspaceRoot: dir, WorkspaceSHA256: digest,
		WorkDir: dir, Timeout: 200 * time.Millisecond, PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{executorPlan("isolated-timeout", source, original, []byte("changed\n"), true)})

	if report.Status != executor.StatusProofBlocked || len(report.Confirmations) != 1 || !report.Confirmations[0].Command.TimedOut {
		t.Fatalf("report = %+v, want isolated timeout blocker", report)
	}
	isolation := report.Confirmations[0].Isolation
	if isolation == nil || !isolation.IsolatedRemoved || !isolation.OriginalIntact || isolation.Error != "" {
		t.Fatalf("timeout isolation evidence = %+v", isolation)
	}
	if _, err := os.Stat(filepath.Join(dir, "child-survived")); !os.IsNotExist(err) {
		t.Fatal("timed-out child mutated the frozen workspace")
	}
	assertFileBytes(t, source, original)
}

func TestCounterexampleExecutorBaselineWitnessConfirmsExactCleanVector(t *testing.T) {
	request := frontendPythonRequest(t, semanticir.ArtifactCode, "def decide(x):\n    if x == 'a':\n        return 'zero'\n    return 'one'\n", []semanticir.Domain{frontendDomain("x", "a", "b")}, "decide")
	request.Workspace.Root = executorCanonicalPath(t, request.Workspace.Root)
	request.Workspace.TreeDigest = executorWorkspaceDigest(t, request.Workspace.Root)
	request.Options = map[string]string{"python.execution": "exhaustive", "python.package_root": ".", "python.module": "code"}
	request.Prover = fakePinnedProver(t)
	endLine := strings.Count(string(request.Source), "\n")
	if !bytes.HasSuffix(request.Source, []byte("\n")) {
		endLine++
	}
	request.ChangedRanges = []semanticir.ChangedSourceRange{{
		ArtifactID: request.Artifact.ID, Path: request.Artifact.Path, StartLine: 1, EndLine: endLine,
		SliceDigest: request.Artifact.Digest,
		Provenance: semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{
			Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: endLine, EndColumn: 1,
		}, semanticir.TranslationTranslated),
	}}
	request.FiniteDomains[0].Type = semanticir.TypeString
	request.Operations[0].Inputs[0].Type = semanticir.TypeString
	request.Groundings = nil
	for index := range request.FiniteDomains[0].Values {
		label := request.FiniteDomains[0].Values[index].ID
		literal := semanticir.Literal{Type: semanticir.TypeString, String: label}
		membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{
			{Kind: semanticir.ExprVariable, Type: semanticir.TypeString, Name: "x"},
			{Kind: semanticir.ExprLiteral, Type: semanticir.TypeString, Literal: &literal},
		}}
		request.FiniteDomains[0].Values[index].Value = nil
		request.FiniteDomains[0].Values[index].Groundings = []semanticir.GroundingAxiom{
			{OperationID: "decide", Kind: semanticir.GroundingMembership, Membership: &membership, ConcreteWitness: map[string]semanticir.Literal{"x": literal}},
		}
		conditions := semanticir.Assignment{request.FiniteDomains[0].ID: label}
		request.Groundings = append(request.Groundings, semanticir.AssignmentGrounding{
			ID: semanticir.AssignmentGroundingID("decide", conditions), OperationID: "decide",
			Conditions: conditions, Inputs: map[string]semanticir.Literal{"x": literal},
		})
	}
	outcomeProvenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	zero := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, OperationID: "decide", Value: &semanticir.Literal{Type: semanticir.TypeString, String: "zero"}, Provenance: outcomeProvenance}
	one := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, OperationID: "decide", Value: &semanticir.Literal{Type: semanticir.TypeString, String: "one"}, Provenance: outcomeProvenance}
	zero.ID, one.ID = semanticir.OutcomeID(zero), semanticir.OutcomeID(one)
	request.Outcomes = []semanticir.ObservableOutcome{zero, one}
	request.Operations[0].OutcomeIDs = []string{zero.ID, one.ID}
	model, diagnostics := frontendpython.Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("fresh baseline translation blocked: %+v", diagnostics)
	}
	for index := range model.ExhaustiveEvidence {
		replayed := executor.ReplayExhaustive(context.Background(), executor.ExhaustiveReplayPlan{
			ID: "baseline-replay", Workspace: executor.ProbeWorkspace{
				ID: request.Workspace.ID, Root: request.Workspace.Root,
				State: semanticir.WorkspaceSolutionNewTests, TreeSHA256: request.Workspace.TreeDigest,
			},
			SourceArtifacts: []semanticir.ArtifactRef{request.Artifact}, Operations: model.Operations,
			Evidence: model.ExhaustiveEvidence[index],
		})
		semanticReplay, replayErr := executor.SemanticReplay(replayed)
		if replayErr != nil {
			t.Fatalf("fresh baseline exhaustive replay blocked: evidence=%+v error=%v", replayed, replayErr)
		}
		model.ExhaustiveEvidence[index].Replay = semanticReplay
	}
	if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
		t.Fatalf("fresh baseline model invalid: %+v", validation)
	}
	choices := make([]semanticir.BehaviorChoice, 0, len(model.Cases))
	for _, behaviorCase := range model.Cases {
		if len(behaviorCase.OutcomeIDs) != 1 {
			t.Fatalf("case %q outcomes = %v, want singleton clean behavior", behaviorCase.ID, behaviorCase.OutcomeIDs)
		}
		choices = append(choices, semanticir.BehaviorChoice{
			Behavior:  semanticir.BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: behaviorCase.Conditions, Provenance: behaviorCase.Provenance},
			OutcomeID: behaviorCase.OutcomeIDs[0],
		})
	}
	sort.Slice(choices, func(i, j int) bool {
		left, _ := semanticir.CanonicalJSON(struct {
			OperationID string                `json:"operation_id"`
			Conditions  semanticir.Assignment `json:"conditions"`
		}{choices[i].Behavior.OperationID, choices[i].Behavior.Conditions})
		right, _ := semanticir.CanonicalJSON(struct {
			OperationID string                `json:"operation_id"`
			Conditions  semanticir.Assignment `json:"conditions"`
		}{choices[j].Behavior.OperationID, choices[j].Behavior.Conditions})
		return string(left) < string(right)
	})
	observed := make([]string, len(choices))
	for index := range choices {
		observed[index] = choices[index].OutcomeID
	}
	witness := semanticir.Counterexample{
		ID: "baseline-equal-witness", Obligation: semanticir.ObligationTestsSound,
		OperationID: choices[0].Behavior.OperationID, Conditions: choices[0].Behavior.Conditions,
		Choices: choices, ObservedOutcomes: observed, TestPasses: true, Provenance: choices[0].Behavior.Provenance,
	}
	modelDigest, _ := semanticir.Digest(model)
	modelCoreDigest, err := semanticir.ArtifactModelTranslationDigest(model)
	if err != nil {
		t.Fatal(err)
	}
	proofDigests := make([]string, 0, len(model.CompilerEvidence)+len(model.ExhaustiveEvidence))
	for _, proof := range model.CompilerEvidence {
		digest, _ := semanticir.Digest(proof)
		proofDigests = append(proofDigests, digest)
	}
	for _, proof := range model.ExhaustiveEvidence {
		digest, _ := semanticir.Digest(proof)
		proofDigests = append(proofDigests, digest)
	}
	candidateDigest, _ := semanticir.Digest(choices)
	testSuiteDigest, _ := semanticir.Digest(semanticir.TestSuiteModel{})
	predicateDigest, _ := semanticir.Digest(semanticir.TestPredicate{})
	verifier := request.Translator
	execution := executor.TaskEnvironment{
		Command: []string{verifier.Path, "-I", "-B", "-c", "pass"}, WorkspaceRoot: request.Workspace.Root,
		WorkspaceSHA256: request.Workspace.TreeDigest, WorkDir: request.Workspace.Root, Timeout: 10 * time.Second,
		Environment: []string{"PYTHONHASHSEED=0"}, ExactEnvironment: true, PassSignal: executor.ExitCodeSignal(0),
	}
	plan := executor.BaselineWitnessPlan{
		ID: "baseline-plan", WitnessID: witness.ID, Obligation: witness.Obligation, Witness: witness,
		Workspace:       executor.ProbeWorkspace{ID: request.Workspace.ID, Root: request.Workspace.Root, State: semanticir.WorkspaceSolutionNewTests, TreeSHA256: request.Workspace.TreeDigest},
		SourceArtifacts: []semanticir.ArtifactRef{request.Artifact}, Translators: []semanticir.ToolRef{request.Translator}, Verifier: verifier,
		Retranslations: []executor.BaselineRetranslationEvidence{{
			ArtifactID: request.Artifact.ID, CandidateSHA256: request.Artifact.Digest, ModelSHA256: modelDigest,
			OriginalModelCoreSHA256: modelCoreDigest, FreshModelCoreSHA256: modelCoreDigest,
			Model: model, Coverage: model.Coverage.Status, ModelProofSHA256: proofDigests,
		}},
		Vector:    executor.BaselineSemanticVector{ID: "clean-solution", Choices: choices, CandidateSHA256: candidateDigest, Baseline: true, TestsPass: true, TestSuiteSHA256: testSuiteDigest, StaticPredicateSHA256: predicateDigest},
		Execution: execution,
	}
	report := executor.ConfirmBaselineWitnesses(context.Background(), execution, []executor.BaselineWitnessPlan{plan})

	if report.Status != executor.StatusConfirmed || len(report.Confirmations) != 1 || report.Confirmations[0].Mode != executor.ConfirmationModeBaselineWitness {
		t.Fatalf("baseline witness report = %+v, want confirmed", report)
	}
	if err := executor.ValidateBaselineWitnessConfirmation(report.Confirmations[0]); err != nil {
		t.Fatalf("certificate-facing baseline witness rejected: %v", err)
	}
	tampered := cloneExecutorConfirmation(t, report.Confirmations[0])
	tampered.BaselineWitness.Plan.Vector.Choices[0].OutcomeID = "detached-outcome"
	if err := executor.ValidateBaselineWitnessConfirmation(tampered); err == nil {
		t.Fatal("tampered baseline semantic vector passed validation")
	}
	if got := executorWorkspaceDigest(t, request.Workspace.Root); got != request.Workspace.TreeDigest {
		t.Fatalf("baseline witness changed frozen workspace: got %s want %s", got, request.Workspace.TreeDigest)
	}
}

type executorProbeFixture struct {
	root            string
	source          string
	originalDigest  string
	workspaceDigest string
	plan            executor.ProbePlan
	baseline        executor.TaskEnvironment
}

func newExecutorProbeFixture(t *testing.T, existingHarness []byte) executorProbeFixture {
	t.Helper()
	root := executorCanonicalPath(t, t.TempDir())
	sourceBody := []byte("solution\n")
	source := writeExecutorFixture(t, root, "subject.txt", sourceBody)
	harnessPath := ".ray/probes/probe.sh"
	if existingHarness != nil {
		if err := os.MkdirAll(filepath.Join(root, ".ray", "probes"), 0o750); err != nil {
			t.Fatal(err)
		}
		writeExecutorFixture(t, root, harnessPath, existingHarness)
	}
	workspaceDigest := executorWorkspaceDigest(t, root)
	shellPath, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	shellPath = executorCanonicalPath(t, shellPath)
	shellBytes, err := os.ReadFile(shellPath)
	if err != nil {
		t.Fatal(err)
	}
	artifact := semanticir.ArtifactRef{ID: "solution-code", Kind: semanticir.ArtifactCode, Path: "subject.txt", Digest: executorTestDigest(sourceBody)}
	provenance := semanticir.Provenance{
		ArtifactID: artifact.ID, ArtifactDigest: artifact.Digest,
		Location:    semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1},
		Translation: semanticir.TranslationTranslated,
	}
	effectValue := semanticir.Expression{
		Kind: semanticir.ExprLiteral, Type: semanticir.TypeString,
		Literal: &semanticir.Literal{Type: semanticir.TypeString, String: "bad-side-effect"}, Provenance: provenance,
	}
	effect := semanticir.Effect{ID: "effect-bad", Kind: semanticir.EffectOutput, Target: "stdout", Value: &effectValue, Provenance: provenance}
	bad := semanticir.ObservableOutcome{
		Kind: semanticir.OutcomeReturn, OperationID: "operation",
		Value: &semanticir.Literal{Type: semanticir.TypeString, String: "bad"}, Effects: []semanticir.Effect{effect}, Provenance: provenance,
	}
	good := semanticir.ObservableOutcome{
		Kind: semanticir.OutcomeReturn, OperationID: "operation",
		Value: &semanticir.Literal{Type: semanticir.TypeString, String: "good"}, Provenance: provenance,
	}
	bad.ID, good.ID = semanticir.OutcomeID(bad), semanticir.OutcomeID(good)
	witness := semanticir.Counterexample{
		ID: "reference-witness", Obligation: semanticir.ObligationReferenceCorrectness,
		Conditions: semanticir.Assignment{"input": "edge"}, OperationID: "operation",
		RequirementID: "requirement", Choices: []semanticir.BehaviorChoice{{
			Behavior:  semanticir.BehaviorRef{OperationID: "operation", Conditions: semanticir.Assignment{"input": "edge"}, Provenance: provenance},
			OutcomeID: bad.ID,
		}},
		ObservedOutcomes: []string{bad.ID}, ExpectedOutcomes: []string{good.ID},
		TestPasses: false, Provenance: provenance,
	}
	rawBad := semanticir.RawOutcomeTrace{
		Kind: semanticir.OutcomeReturn, Value: &semanticir.Literal{Type: semanticir.TypeString, String: "bad"},
		Effects: []semanticir.RawEffectTrace{{Kind: semanticir.EffectOutput, Target: "stdout", Value: &semanticir.Literal{Type: semanticir.TypeString, String: "bad-side-effect"}}},
	}
	expected := semanticir.ExpectedSemantics{
		Conditions: witness.Conditions, OperationID: witness.OperationID,
		OutcomeIDs: append([]string(nil), witness.ObservedOutcomes...), Choices: append([]semanticir.BehaviorChoice(nil), witness.Choices...), TestPasses: witness.TestPasses,
		RuntimeOutcomes: []semanticir.RuntimeOutcomeChoice{{Behavior: witness.Choices[0].Behavior, RawOutcome: rawBad, MappingOutcomeID: bad.ID}},
	}
	harness := []byte("#!/bin/sh\nset -eu\ntest \"$(cat subject.txt)\" = solution\nprintf '%s' \"$1\" > .ray/probes/probe-observation.json\nprintf 'probe stdout'\nprintf 'probe stderr' >&2\n")
	shellRef := semanticir.ToolRef{Name: "shell", Path: shellPath, Digest: executorTestDigest(shellBytes), Version: "frozen-test-shell"}
	plan := executor.ProbePlan{
		ID: "reference-probe", WitnessID: witness.ID, Obligation: semanticir.ObligationReferenceCorrectness,
		Witness: witness, SourceArtifacts: []semanticir.ArtifactRef{artifact},
		Workspace:  executor.ProbeWorkspace{ID: "solution-new-tests", Root: root, State: semanticir.WorkspaceSolutionNewTests, TreeSHA256: workspaceDigest},
		Tools:      []semanticir.ToolRef{shellRef},
		Operations: []semanticir.Operation{{ID: "operation", Kind: semanticir.OperationFunction, OutcomeIDs: []string{bad.ID, good.ID}, Provenance: provenance}},
		Harness:    executor.ProbeHarness{Path: harnessPath, Bytes: harness, SHA256: executorTestDigest(harness), Mode: 0o700},
		Steps: []executor.ProbeStep{{
			ID: "run", Kind: executor.ProbeStepRun, Tool: &shellRef,
			Argv: []string{shellPath, harnessPath}, WorkDir: ".", Timeout: time.Second,
			PassSignal: executor.ExitCodeSignal(0), ObservationPath: ".ray/probes/probe-observation.json",
		}},
		ExpectedSemantics: expected,
	}
	setExecutorProbeObservation(t, &plan, executor.ProbeObservation{Traces: []semanticir.RawOutcomeTrace{rawBad}})
	return executorProbeFixture{
		root: root, source: source, originalDigest: executorTestDigest(sourceBody), workspaceDigest: workspaceDigest, plan: plan,
		baseline: executor.TaskEnvironment{
			Command:       []string{shellPath, "-c", "test \"$(cat subject.txt)\" = solution"},
			WorkspaceRoot: root, WorkDir: root, Timeout: time.Second, PassSignal: executor.ExitCodeSignal(0),
		},
	}
}

func setExecutorProbeObservation(t *testing.T, plan *executor.ProbePlan, observed executor.ProbeObservation) {
	t.Helper()
	body, err := json.Marshal(observed)
	if err != nil {
		t.Fatal(err)
	}
	step := &plan.Steps[len(plan.Steps)-1]
	if len(step.Argv) == 2 {
		step.Argv = append(step.Argv, string(body))
	} else {
		step.Argv[2] = string(body)
	}
}

func cloneProbeObservation(t *testing.T, observation executor.ProbeObservation) executor.ProbeObservation {
	t.Helper()
	body, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	var clone executor.ProbeObservation
	if err := json.Unmarshal(body, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func cloneExecutorConfirmation(t *testing.T, confirmation executor.Confirmation) executor.Confirmation {
	t.Helper()
	body, err := json.Marshal(confirmation)
	if err != nil {
		t.Fatal(err)
	}
	var clone executor.Confirmation
	if err := json.Unmarshal(body, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func executorWorkspaceDigest(t *testing.T, root string) string {
	t.Helper()
	digest, err := executor.WorkspaceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func executorCanonicalPath(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(resolved)
}

func assertProbeWorkspaceUnchanged(t *testing.T, fixture executorProbeFixture) {
	t.Helper()
	if got := executorWorkspaceDigest(t, fixture.root); got != fixture.workspaceDigest {
		t.Fatalf("workspace digest = %s, want frozen %s", got, fixture.workspaceDigest)
	}
	assertFileBytes(t, fixture.source, []byte("solution\n"))
}

func testExecutorRestoration(t *testing.T, name, scriptBody string, expectedPass bool, configure func(*executor.TaskEnvironment, context.CancelFunc)) {
	t.Helper()
	dir := t.TempDir()
	original := []byte("clean\n\x00byte-exact\r\n")
	source := writeExecutorFixture(t, dir, "subject.txt", original)
	script := writeExecutorScript(t, dir, scriptBody)
	plan := executorPlan(name+"-plan", source, original, []byte("counterexample\n"), expectedPass)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	task := executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: dir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}
	if configure != nil {
		configure(&task, cancel)
	}
	report := executor.Confirm(ctx, task, []semanticir.EditPlan{plan})
	if len(report.Confirmations) != 1 {
		t.Fatalf("confirmations = %d, want 1: %+v", len(report.Confirmations), report)
	}
	confirmation := report.Confirmations[0]
	if len(confirmation.Materializations) != 1 || !confirmation.Materializations[0].Applied || !confirmation.Materializations[0].Restored {
		t.Fatalf("materialization evidence = %+v", confirmation.Materializations)
	}
	if name == "timeout" {
		if report.Status != executor.StatusProofBlocked || !confirmation.Command.TimedOut {
			t.Fatalf("report = %+v, want timeout blocker", report)
		}
	} else if report.Status != executor.StatusConfirmed {
		t.Fatalf("report = %+v, want confirmed", report)
	}
	assertFileBytes(t, source, original)
}

func executorPlan(id, source string, original, replacement []byte, expectedPass bool) semanticir.EditPlan {
	artifact := semanticir.ArtifactRef{
		ID: "code-artifact", Kind: semanticir.ArtifactCode, Path: source, Digest: executorTestDigest(original),
	}
	return semanticir.EditPlan{
		ID: id, WitnessID: "witness-" + id, Artifact: artifact,
		Edits: []semanticir.ByteRangeReplacement{{
			StartByte: 0, EndByte: len(original), ExpectedBytes: append([]byte(nil), original...),
			Replacement: append([]byte(nil), replacement...),
		}},
		Expected: semanticir.ExpectedSemantics{
			OperationID: "operation", OutcomeIDs: []string{"outcome"}, TestPasses: expectedPass,
		},
		Provenance: semanticir.Provenance{
			ArtifactID: artifact.ID, ArtifactDigest: artifact.Digest, Translation: semanticir.TranslationTranslated,
		},
	}
}

func assertExecutorPathBlocked(t *testing.T, workDir, declaredPath, actualPath string, original []byte) {
	t.Helper()
	script := writeExecutorScript(t, workDir, "touch verifier-ran\nexit 0\n")
	plan := executorPlan("path-escape", actualPath, original, []byte("changed\n"), true)
	plan.Artifact.Path = declaredPath
	report := executor.Confirm(context.Background(), executor.TaskEnvironment{
		Command: []string{"sh", script}, WorkDir: workDir, Timeout: time.Second,
		PassSignal: executor.ExitCodeSignal(0),
	}, []semanticir.EditPlan{plan})

	if report.Status != executor.StatusProofBlocked || !hasExecutorBlocker(report.Blockers, "artifact-outside-workdir") {
		t.Fatalf("report = %+v, want artifact-outside-workdir blocker", report)
	}
	if report.Baseline.StartedAt != (time.Time{}) {
		t.Fatalf("baseline started for escaped artifact: %+v", report.Baseline)
	}
	if _, err := os.Stat(filepath.Join(workDir, "verifier-ran")); !os.IsNotExist(err) {
		t.Fatal("verifier ran for an artifact outside the frozen workspace")
	}
	assertFileBytes(t, actualPath, original)
}

func executorBindPlanProvenance(plan *semanticir.EditPlan) {
	plan.Provenance = semanticir.Provenance{
		ArtifactID: plan.Artifact.ID, ArtifactDigest: plan.Artifact.Digest,
		Translation: semanticir.TranslationTranslated,
	}
}

func executorTestDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writeExecutorFixture(t *testing.T, dir, name string, body []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeExecutorScript(t *testing.T, dir, body string) string {
	t.Helper()
	return writeExecutorFixture(t, dir, fmt.Sprintf("verifier-%d.sh", time.Now().UnixNano()), []byte(strings.TrimSpace(body)+"\n"))
}

func assertConfirmedExecutorReport(t *testing.T, report executor.Report) {
	t.Helper()
	if report.Status != executor.StatusConfirmed {
		t.Fatalf("status = %q, want %q; report = %+v", report.Status, executor.StatusConfirmed, report)
	}
	if !report.Baseline.Passed || len(report.Confirmations) != 1 {
		t.Fatalf("baseline/confirmations = %+v/%d, want passing baseline and one confirmation", report.Baseline, len(report.Confirmations))
	}
	if report.Confirmations[0].Status != executor.StatusConfirmed {
		t.Fatalf("confirmation = %+v, want confirmed", report.Confirmations[0])
	}
}

func assertFileBytes(t *testing.T, path string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("%s bytes = %q, want %q", path, actual, expected)
	}
}

func waitForExecutorFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func hasExecutorBlocker(blockers []executor.Blocker, code string) bool {
	for _, blocker := range blockers {
		if blocker.Code == code {
			return true
		}
	}
	return false
}
