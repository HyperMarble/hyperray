package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func TestReplayDerivationConfirmsBothExactStreamsTwice(t *testing.T) {
	plan := derivationReplayFixture(t, "stable", "")
	evidence := ReplayDerivation(context.Background(), plan)
	if evidence.Status != StatusConfirmed || !evidence.Deterministic || len(evidence.Blockers) != 0 || len(evidence.Runs) != 2 {
		t.Fatalf("evidence = %+v, want deterministic confirmed replay", evidence)
	}
	if evidence.Runs[0].Isolation.IsolatedRoot == evidence.Runs[1].Isolation.IsolatedRoot {
		t.Fatal("derivation repetitions reused one disposable workspace")
	}
	for _, run := range evidence.Runs {
		if !run.Complete || !run.Isolation.IsolatedRemoved || !run.Isolation.OriginalIntact || !reflect.DeepEqual(run.IR, plan.Graph.IR) || !reflect.DeepEqual(run.DecoderOutput, plan.Graph.DecoderOutput) || len(run.DerivationSteps) != 1 || len(run.DecoderSteps) != 1 {
			t.Fatalf("incomplete derivation run: %+v", run)
		}
	}
	if err := ValidateDerivationReplay(evidence); err != nil {
		t.Fatalf("confirmed derivation replay rejected: %v", err)
	}
	tampered := cloneDerivationReplay(t, evidence)
	tampered.Runs[0].DecoderOutput = []byte(`[{"semantic_id":"forged"}]`)
	if err := ValidateDerivationReplay(tampered); err == nil {
		t.Fatal("tampered decoder output passed replay validation")
	}
}

func TestReplayDerivationRejectsStaleAndForgedBindings(t *testing.T) {
	t.Run("stale-source", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		plan.SourceArtifacts[0].Digest = digestBytes([]byte("different source"))
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "stale-derivation-source")
		if len(evidence.Runs) != 0 {
			t.Fatal("stale source reached derivation execution")
		}
	})

	t.Run("stale-tool", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		forged := digestBytes([]byte("different tool"))
		plan.Graph.Tool.Digest = forged
		plan.Graph.DerivationSteps[0].Tool.Digest = forged
		plan.Graph.DecoderSteps[0].Tool.Digest = forged
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "invalid-derivation-tool")
	})

	t.Run("forged-ir", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		plan.Graph.IR = []byte("frontend-forged-ir\n")
		plan.Graph.IRDigest = digestBytes(plan.Graph.IR)
		plan.Graph.DerivationSteps[0].ExpectedStdoutDigest = plan.Graph.IRDigest
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "derivation-replay-mismatch")
	})

	t.Run("process-supplied-unknown-decoder-field", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		plan.Graph.DecoderSteps[0].Argv[len(plan.Graph.DecoderSteps[0].Argv)-1] = `[{"semantic_id":"forged"}]`
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "derivation-replay-mismatch")
	})

	t.Run("frontend-supplied-unknown-decoder-field", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		plan.Graph.DecoderOutput = []byte(`[{"semantic_id":"forged"}]`)
		plan.Graph.DecoderOutputDigest = digestBytes(plan.Graph.DecoderOutput)
		plan.Graph.DecoderSteps[0].ExpectedStdoutDigest = plan.Graph.DecoderOutputDigest
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "invalid-derivation-decoder")
		if len(evidence.Runs) != 0 {
			t.Fatal("unknown decoder field reached execution")
		}
	})
}

func TestReplayDerivationRejectsCrossEnvironmentAndNondeterminism(t *testing.T) {
	t.Run("cross-environment", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		plan.Graph.DecoderSteps[0].Environment = []semanticir.EnvironmentVariable{{Name: "RAY_DERIVATION_HELPER", Value: "different"}}
		plan.Graph.DecoderSteps[0].EnvironmentDigest, _ = semanticir.Digest(plan.Graph.DecoderSteps[0].Environment)
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "cross-derivation-environment")
		if len(evidence.Runs) != 0 {
			t.Fatal("cross-environment transcript reached execution")
		}
	})

	t.Run("nondeterministic-second-repetition", func(t *testing.T) {
		counter := filepath.Join(t.TempDir(), "counter")
		plan := derivationReplayFixture(t, "nondeterministic", counter)
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "derivation-replay-mismatch")
		if len(evidence.Runs) != 2 || !evidence.Runs[0].Complete || evidence.Runs[1].Complete || evidence.Deterministic {
			t.Fatalf("nondeterministic repetitions were not detected independently: %+v", evidence.Runs)
		}
	})
}

func TestReplayDerivationRejectsCrossTranscriptAliases(t *testing.T) {
	t.Run("duplicate-step-id", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		plan.Graph.DecoderSteps[0].ID = plan.Graph.DerivationSteps[0].ID
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "invalid-derivation-steps")
		if len(evidence.Runs) != 0 {
			t.Fatal("duplicate cross-transcript step ID reached execution")
		}
	})

	t.Run("duplicate-output-path", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		output := semanticir.ProbeOutput{
			ID: "decoder-output", Path: ".hyperray/derivation/shared-output",
			AfterDigest: plan.Graph.Tool.Digest, Executable: true,
			Provenance: plan.Graph.Provenance,
		}
		deriveOutput := output
		deriveOutput.ID = "derive-output"
		deriveSetup := plan.Graph.DerivationSteps[0]
		deriveSetup.ID = "derive-setup"
		deriveSetup.Kind = semanticir.ProbeStepSetup
		deriveSetup.ExpectedStdoutDigest = digestBytes(nil)
		deriveSetup.Outputs = []semanticir.ProbeOutput{deriveOutput}
		plan.Graph.DerivationSteps = append([]semanticir.ProbeStep{deriveSetup}, plan.Graph.DerivationSteps...)
		decoderSetup := plan.Graph.DecoderSteps[0]
		decoderSetup.ID = "decoder-setup"
		decoderSetup.Kind = semanticir.ProbeStepSetup
		decoderSetup.ExpectedStdoutDigest = digestBytes(nil)
		decoderSetup.Outputs = []semanticir.ProbeOutput{output}
		plan.Graph.DecoderSteps = append([]semanticir.ProbeStep{decoderSetup}, plan.Graph.DecoderSteps...)
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "duplicate-derivation-output")
		if len(evidence.Runs) != 0 {
			t.Fatal("aliased generated output path reached execution")
		}
	})
}

func TestReplayDerivationCanonicalContainment(t *testing.T) {
	t.Run("root-alias", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "nested"), 0o750); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(t.TempDir(), "workspace-alias")
		if err := os.Symlink(root, alias); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		path, err := resolveProbeParent(alias, "nested/result.json")
		canonicalRoot, resolveErr := filepath.EvalSymlinks(root)
		if err != nil || resolveErr != nil || filepath.Dir(path) != filepath.Join(canonicalRoot, "nested") {
			t.Fatalf("canonical alias rejected: path=%q err=%v", path, err)
		}
	})

	t.Run("sibling-prefix", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "workspace")
		sibling := filepath.Join(parent, "workspace-sibling")
		if err := os.Mkdir(root, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(sibling, 0o750); err != nil {
			t.Fatal(err)
		}
		if pathWithin(root, sibling) {
			t.Fatal("sibling with common path prefix passed containment")
		}
	})

	t.Run("symlink-parent-escape", func(t *testing.T) {
		plan := derivationReplayFixture(t, "stable", "")
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(plan.Workspace.Root, "escape")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		plan.Workspace.TreeSHA256, _ = WorkspaceDigest(plan.Workspace.Root)
		plan.Graph.WorkspaceTreeDigest = plan.Workspace.TreeSHA256
		plan.Graph.DecoderSteps[0].WorkingDirectory = filepath.Join(plan.Workspace.Root, "escape")
		evidence := ReplayDerivation(context.Background(), plan)
		assertDerivationBlocked(t, evidence, "invalid-derivation-steps")
	})
}

func TestDerivationReplayHelperProcess(t *testing.T) {
	if os.Getenv("RAY_DERIVATION_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || len(os.Args) < separator+4 || os.Args[separator+1] != "emit" {
		os.Exit(92)
	}
	mode, output := os.Args[separator+2], os.Args[separator+3]
	if mode == "nondeterministic" && strings.HasPrefix(output, "compiler-ir") {
		counter := os.Getenv("RAY_DERIVATION_COUNTER")
		if counter == "" {
			os.Exit(93)
		}
		if _, err := os.Stat(counter); os.IsNotExist(err) {
			if err := os.WriteFile(counter, []byte("seen"), 0o600); err != nil {
				os.Exit(94)
			}
		} else {
			output = fmt.Sprintf("nondeterministic-%d\n", os.Getpid())
		}
	}
	_, _ = os.Stdout.Write([]byte(output))
	os.Exit(0)
}

func derivationReplayFixture(t *testing.T, mode, counter string) DerivationReplayPlan {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sourceBody := []byte("def choose(x): return x\n")
	sourcePath := filepath.Join(root, "subject.py")
	if err := os.WriteFile(sourcePath, sourceBody, 0o640); err != nil {
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
	tool := semanticir.ToolRef{Name: "derivation-test-helper", Path: toolPath, Digest: digestBytes(toolBytes), Version: "test-binary-v1"}
	source := semanticir.ArtifactRef{ID: "source", Path: "subject.py", Digest: digestBytes(sourceBody), Kind: semanticir.ArtifactCode}
	provenance := semanticir.Provenance{
		ArtifactID: source.ID, ArtifactDigest: source.Digest,
		Location:    semanticir.SourceLocation{Path: source.Path, StartLine: 1, StartColumn: 1},
		Translation: semanticir.TranslationTranslated,
	}
	environment := []semanticir.EnvironmentVariable{{Name: "RAY_DERIVATION_HELPER", Value: "1"}}
	if counter != "" {
		environment = append(environment, semanticir.EnvironmentVariable{Name: "RAY_DERIVATION_COUNTER", Value: counter})
		sortEnvironmentVariables(environment)
	}
	environmentDigest, _ := semanticir.Digest(environment)
	emptyDigest := digestBytes(nil)
	ir := []byte("compiler-ir\n")
	step := func(id, output string) semanticir.ProbeStep {
		return semanticir.ProbeStep{
			ID: id, Kind: semanticir.ProbeStepRun, Tool: tool,
			Argv: append(derivationHelperInvocation(), "emit", mode, output), StdinDigest: emptyDigest,
			WorkingDirectory: root, Environment: append([]semanticir.EnvironmentVariable(nil), environment...), EnvironmentDigest: environmentDigest,
			ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: int64((5 * time.Second) / time.Millisecond), ExpectedExitCode: 0,
			ExpectedStdoutDigest: digestBytes([]byte(output)), ExpectedStderrDigest: emptyDigest, ExpectedSignalDigest: emptyDigest,
			SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: provenance,
		}
	}
	workspaceDigest, err := WorkspaceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	graph := semanticir.CompilerSemanticGraph{
		SourceDigest: source.Digest, WorkspaceTreeDigest: workspaceDigest, Tool: tool,
		IRKind: semanticir.CompilerIRCPythonBytecode, IR: ir, IRDigest: digestBytes(ir),
		Environment: environment, EnvironmentDigest: environmentDigest,
		DerivationSteps: []semanticir.ProbeStep{step("derive", string(ir))},
		Constructs:      []semanticir.CompilerConstructBinding{}, Provenance: provenance,
	}
	decoder, err := semanticir.CanonicalCompilerDecoderOutput(&graph)
	if err != nil {
		t.Fatal(err)
	}
	graph.DecoderSteps = []semanticir.ProbeStep{step("decode", string(decoder))}
	graph.DecoderOutput = decoder
	graph.DecoderOutputDigest = digestBytes(decoder)
	return DerivationReplayPlan{
		ID: "derivation-replay", Workspace: ProbeWorkspace{ID: "solution-new-tests", Root: root, State: semanticir.WorkspaceSolutionNewTests, TreeSHA256: workspaceDigest},
		SourceArtifacts: []semanticir.ArtifactRef{source}, Graph: graph,
	}
}

func derivationHelperInvocation() []string {
	return []string{"-test.run=^TestDerivationReplayHelperProcess$", "--"}
}

func sortEnvironmentVariables(environment []semanticir.EnvironmentVariable) {
	for left := range environment {
		for right := left + 1; right < len(environment); right++ {
			if environment[right].Name < environment[left].Name {
				environment[left], environment[right] = environment[right], environment[left]
			}
		}
	}
}

func cloneDerivationReplay(t *testing.T, evidence DerivationReplayEvidence) DerivationReplayEvidence {
	t.Helper()
	body, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	var result DerivationReplayEvidence
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertDerivationBlocked(t *testing.T, evidence DerivationReplayEvidence, code string) {
	t.Helper()
	if evidence.Status != StatusProofBlocked {
		t.Fatalf("status = %q, want proof-blocked: %+v", evidence.Status, evidence)
	}
	for _, blocker := range evidence.Blockers {
		if blocker.Code == code {
			return
		}
	}
	t.Fatalf("blockers = %+v, want code %q", evidence.Blockers, code)
}
