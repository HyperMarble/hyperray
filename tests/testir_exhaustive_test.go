package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/testir"
)

type testIRFixture struct {
	request testir.Request
	root    string
}

type fakeMaterializeMode string

const (
	materializeExact     fakeMaterializeMode = "exact"
	materializeStale     fakeMaterializeMode = "stale"
	materializeNoop      fakeMaterializeMode = "noop"
	materializeIncorrect fakeMaterializeMode = "incorrect"
)

func TestTestIRRelational(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 2)
	assertPredicateTruth(t, result.Predicate, result.Vectors)
	accepted := acceptedVectors(result)
	if strings.Join(accepted, ",") != "00,11" {
		t.Fatalf("relational verifier accepted %v, want [00 11]", accepted)
	}
}

func TestTestIRExhaustive(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	result := testir.CrossCheck(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 2)
	seen := map[string]bool{}
	for _, vector := range result.Vectors {
		seen[compactVector(vector.Choices)] = true
	}
	for _, vector := range []string{"00", "01", "10", "11"} {
		if !seen[vector] {
			t.Fatalf("exact executable cross-check omitted behavior vector %s", vector)
		}
	}
}

func TestTestIRNoTests(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "all", materializeExact)
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 4)
	assertPredicateTruth(t, result.Predicate, result.Vectors)
}

func TestTestIRMissingTest(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "first-zero", materializeExact)
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 2)
	// The spec requires zero for both cases. 01 passes the verifier and
	// violates the second requirement: this is the exact false-positive shape.
	found := false
	for _, vector := range result.Vectors {
		if compactVector(vector.Choices) == "01" && vector.TestsPass {
			found = true
		}
	}
	if !found {
		t.Fatal("missing second-case test did not expose passing wrong vector 01")
	}
}

func TestTestIRUnfairFailure(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	// Make every outcome allowed by Spec. The executable test still rejects
	// 01 and 10, proving Spec && !TestsPass is satisfiable.
	for index := range fixture.request.Task.Requirements {
		fixture.request.Task.Requirements[index].RequiredOutcomes = []string{"zero", "one"}
		fixture.request.Task.Requirements[index].ForbiddenOutcomes = nil
	}
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 2)
	found := false
	for _, vector := range result.Vectors {
		if compactVector(vector.Choices) == "01" && !vector.TestsPass {
			found = true
		}
	}
	if !found {
		t.Fatal("unfair equality-only verifier did not reject allowed vector 01")
	}
}

func TestTestIRMultiFileAtomic(t *testing.T) {
	fixture := newMultiArtifactTestIRFixture(t)
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 16, 4)
	var wantedID string
	for _, vector := range result.Vectors {
		if compactVector(vector.Choices) == "1111" {
			wantedID = vector.ID
			if len(vector.Plans) != 2 {
				t.Fatalf("1111 vector has %d plans, want two atomic artifact plans", len(vector.Plans))
			}
		}
	}
	if wantedID == "" {
		t.Fatal("missing 1111 vector")
	}
	confirmations := 0
	for _, confirmation := range result.Execution.Confirmations {
		if confirmation.WitnessID == wantedID {
			confirmations++
			if len(confirmation.Materializations) != 2 {
				t.Fatalf("atomic confirmation contains %d materializations, want two", len(confirmation.Materializations))
			}
			if confirmation.ObservedTestPasses == nil || !*confirmation.ObservedTestPasses {
				t.Fatal("atomic 1111 candidate did not reach the verifier as one passing state")
			}
		}
	}
	if confirmations != 1 {
		t.Fatalf("11 vector executed %d times, want exactly once", confirmations)
	}
}

func TestTestIRRejectsStale(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeStale)
	result := testir.Build(context.Background(), fixture.request)
	assertBlockedTestIR(t, result, "stale-artifact")
}

func TestTestIRRejectsNoop(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeNoop)
	result := testir.Build(context.Background(), fixture.request)
	assertBlockedTestIR(t, result, "no-op-candidate")
}

func TestTestIRRejectsIncorrect(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeIncorrect)
	result := testir.Build(context.Background(), fixture.request)
	assertBlockedTestIR(t, result, "incorrect-candidate")
}

func TestTestIRRejectsWitnessOnlyCategory(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	frozenDigest := fixture.request.Artifacts[0].Frontend.Artifact.Digest
	translate := fixture.request.Artifacts[0].Translate
	fixture.request.Artifacts[0].Translate = func(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic) {
		model, diagnostics := translate(ctx, request)
		if request.Artifact.Digest != frozenDigest {
			// Cases still describe each concrete witness, but the frontend has
			// omitted the proof that those outcomes hold over whole categories.
			model.CompilerEvidence = nil
		}
		return model, diagnostics
	}
	result := testir.Build(context.Background(), fixture.request)
	assertBlockedTestIR(t, result, "category-realization-unproved")
}

func TestTestIRRejectsUnreplayableCategoryProof(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	frozenDigest := fixture.request.Artifacts[0].Frontend.Artifact.Digest
	translate := fixture.request.Artifacts[0].Translate
	fixture.request.Artifacts[0].Translate = func(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic) {
		model, diagnostics := translate(ctx, request)
		if request.Artifact.Digest != frozenDigest {
			for evidenceIndex := range model.CompilerEvidence {
				for proofIndex := range model.CompilerEvidence[evidenceIndex].BehaviorProofs {
					// The record remains structurally canonical, but its exact
					// invocation cannot execute. Replay must catch that.
					model.CompilerEvidence[evidenceIndex].BehaviorProofs[proofIndex].RealizationProof.WorkingDirectory = "/ray-testir-path-that-does-not-exist"
				}
			}
		}
		return model, diagnostics
	}
	result := testir.Build(context.Background(), fixture.request)
	assertBlockedTestIR(t, result, "category-proof-replay-failed")
}

func TestTestIRRejectsSemanticWorkspaceSideEffects(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	frozenDigest := fixture.request.Artifacts[0].Frontend.Artifact.Digest
	translate := fixture.request.Artifacts[0].Translate
	fixture.request.Artifacts[0].Translate = func(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic) {
		model, diagnostics := translate(ctx, request)
		if request.Artifact.Digest != frozenDigest {
			if err := os.WriteFile(filepath.Join(request.Workspace.Root, "translator-cache"), []byte("side effect"), 0o600); err != nil {
				t.Fatalf("create adversarial translator side effect: %v", err)
			}
		}
		return model, diagnostics
	}
	result := testir.Build(context.Background(), fixture.request)
	assertBlockedTestIR(t, result, "semantic-isolation-failed")
	if _, err := os.Stat(filepath.Join(fixture.root, "translator-cache")); !os.IsNotExist(err) {
		t.Fatal("isolated translator side effect escaped into frozen workspace")
	}
}

func TestTestIRAllowsConjoinedRequirementsForOneCase(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	copy := fixture.request.Task.Requirements[0]
	copy.ID += "-second-clause"
	fixture.request.Task.Requirements = append(fixture.request.Task.Requirements, copy)
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 2)
}

func TestTestIRBlocksNondeterministicVerifier(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	counter := filepath.Join(t.TempDir(), "nondeterminism-counter")
	script := fmt.Sprintf(`a=$(awk -F'|' '$2=="x=a" {print $3}' impl.txt); b=$(awk -F'|' '$2=="x=b" {print $3}' impl.txt); if test "$a" = "$b"; then exit 0; fi; p=%q-$a$b; n=0; test ! -f "$p" || n=$(cat "$p"); echo $((n+1)) > "$p"; test $((n%%2)) -eq 1`, counter)
	fixture.request.Executor.Command = []string{"/bin/sh", "-c", script}
	result := testir.Build(context.Background(), fixture.request)
	// A changing verifier can be rejected either when a later repetition
	// differs or earlier when one run already differs from static T(F). Both are
	// proof blockers; no run is selected as authoritative.
	assertBlockedTestIR(t, result, "model-execution-mismatch")
}

func TestTestIRSupportsCommandSubdirectory(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	runDirectory := filepath.Join(fixture.root, "run")
	if err := os.Mkdir(runDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture.request.Executor.WorkspaceRoot = fixture.root
	fixture.request.Executor.WorkDir = runDirectory
	fixture.request.Executor.Command = verifierCommand("equal", "../impl.txt", "")
	fixture.request.Artifacts[0].Frontend.Workspace.WorkingDirectory = "run"
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 2)
	for _, vector := range result.Vectors {
		relative, err := filepath.Rel(vector.ExecutionIsolation.IsolatedRoot, vector.Command.WorkDir)
		if err != nil || relative != "run" {
			t.Fatalf("vector command workdir=%q is not run under isolated root %q", vector.Command.WorkDir, vector.ExecutionIsolation.IsolatedRoot)
		}
	}
}

func TestTestIRBlocksStaticExecutionMismatch(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	fixture.request.Executor.Command = verifierCommand("all", "impl.txt", "")
	result := testir.Build(context.Background(), fixture.request)
	assertBlockedTestIR(t, result, "model-execution-mismatch")
}

func TestTestIRCompileSuite(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 2)
	binding := fakeSuiteBinding(t, fixture)
	suite, err := testir.CompileSuite(result, binding)
	if err != nil {
		t.Fatalf("CompileSuite: %v", err)
	}
	if suite.VectorCount != 4 || suite.CrossCheck == nil || !suite.CrossCheck.Full || suite.CrossCheck.AcceptedVectorCount != 2 ||
		suite.CrossCheck.Repetitions != result.Repetitions || suite.CrossCheck.VectorEvidenceDigest != result.RunDigests[0] || len(suite.SourceModels) != 1 {
		t.Fatalf("compiled suite does not preserve static authority plus advisory cross-check: %+v", suite)
	}
}

func TestTestIRCompileStaticWithoutEnumeration(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	suite := compileFixtureStatic(t, fixture)
	if suite.Predicate.Kind != semanticir.PredicateOutcomeEqual || suite.VectorCount != 4 || suite.CrossCheck != nil || len(suite.Vectors) != 0 || suite.Repetitions != 0 {
		t.Fatalf("static suite unexpectedly depends on exhaustive executions: %+v", suite)
	}
}

func TestTestIRSymbolic(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	suite := compileFixtureStatic(t, fixture)
	if suite.VectorCount != 4 || suite.Predicate.Kind != semanticir.PredicateOutcomeEqual {
		t.Fatalf("exact symbolic Test IR lost its complete vector space or relation: %+v", suite)
	}
	if suite.CrossCheck != nil || len(suite.Vectors) != 0 {
		t.Fatal("symbolic Test IR incorrectly required sampled or enumerated verifier runs")
	}
}

func TestTestIRIntersection(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	model := fixture.request.TestModels[0]
	equality := model.Tests[0].Predicate
	left := *equality.Left
	firstZero := semanticir.Observation{
		Kind: semanticir.ObserveOutcome, Behavior: left, OutcomeIDs: []string{"zero"}, Provenance: equality.Provenance,
	}
	model.Tests = []semanticir.TestModel{
		{ID: "test-equality", Predicate: equality, Provenance: equality.Provenance},
		{ID: "test-first-zero", Predicate: semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &firstZero, Provenance: equality.Provenance}, Provenance: equality.Provenance},
	}
	replaceFixtureTestModel(t, &fixture, model)
	suite := compileFixtureStatic(t, fixture)
	if suite.Predicate.Kind != semanticir.PredicateAnd || len(suite.Predicate.Children) != 2 {
		t.Fatalf("two independently translated tests were not globally conjoined: %+v", suite.Predicate)
	}
	points, diagnostics := semanticir.ConcreteBehaviorPoints(fixture.request.Task)
	if semanticir.HasErrors(diagnostics) || len(points) != 2 {
		t.Fatalf("concrete points: %+v %+v", points, diagnostics)
	}
	for _, testCase := range []struct {
		outcomes []string
		want     bool
	}{
		{[]string{"zero", "zero"}, true},
		{[]string{"zero", "one"}, false},
		{[]string{"one", "one"}, false},
	} {
		choices := []semanticir.BehaviorChoice{{Behavior: points[0], OutcomeID: testCase.outcomes[0]}, {Behavior: points[1], OutcomeID: testCase.outcomes[1]}}
		got, err := proof.EvaluateTestPredicate(fixture.request.Task, suite.Predicate, choices)
		if err != nil || got != testCase.want {
			t.Fatalf("intersection(%v)=%t, want %t (err=%v)", testCase.outcomes, got, testCase.want, err)
		}
	}
}

func TestTestIRGlobalOrderingStateInteraction(t *testing.T) {
	fixture := newMultiArtifactTestIRFixture(t)
	model := fixture.request.TestModels[0]
	global := model.Tests[0].Predicate
	if global.Kind != semanticir.PredicateAnd || len(global.Children) != 2 {
		t.Fatalf("fixture lost its two cross-operation relations: %+v", global)
	}
	model.Tests = []semanticir.TestModel{
		{ID: "test-first-interaction", Predicate: global.Children[0], Provenance: global.Provenance},
		{ID: "test-second-interaction", Predicate: global.Children[1], Provenance: global.Provenance},
	}
	replaceFixtureTestModel(t, &fixture, model)
	binding := fakeSuiteBinding(t, fixture)
	makeFakeOrderedStatefulRunner(t, &binding)
	suite, err := testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: binding})
	if err != nil {
		t.Fatalf("CompileStatic ordered/stateful: %v", err)
	}
	if suite.Predicate.Kind != semanticir.PredicateAnd || len(suite.Predicate.Children) != 2 || suite.VectorCount != 16 {
		t.Fatalf("global cross-operation verifier logic was flattened or localized: %+v", suite)
	}
	if suite.RunnerComposition.Kind != semanticir.RunnerCompositionOrderedStateful || len(suite.RunnerComposition.States) != 1 || len(suite.RunnerComposition.Events) != 2 {
		t.Fatalf("global runner lost ordered shared-state evidence: %+v", suite.RunnerComposition)
	}
	for index, event := range suite.RunnerComposition.Events {
		if event.Ordinal != index || len(event.ReadsStateIDs) != 1 || len(event.WritesStateIDs) != 1 {
			t.Fatalf("ordered shared-state event %d is incomplete: %+v", index, event)
		}
	}
	for _, relation := range suite.Predicate.Children {
		if relation.Kind != semanticir.PredicateOutcomeEqual || relation.Left == nil || relation.Right == nil ||
			relation.Left.OperationID == relation.Right.OperationID || semanticir.BehaviorRefKey(*relation.Left) == semanticir.BehaviorRefKey(*relation.Right) {
			t.Fatalf("ordered shared-state interaction lost distinct operation/case references: %+v", relation)
		}
	}
}

func TestTestIRGlobalLocalAnalysisCommandsDoNotReplacePassSignal(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	model := fixture.request.TestModels[0]
	local := model.RunnerSelection.Command
	local.ID = "local-compiler-analysis"
	local.Command = "fake-local-test-selection --list"
	model.RunnerSelection.Command = local
	fixture.request.TestModels[0] = model
	replaceAttachedTestArtifact(fixture.request.Task, model)
	binding := fakeSuiteBinding(t, fixture)
	suite, err := testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: binding})
	if err != nil {
		t.Fatalf("distinct local analysis command incorrectly replaced/blocked the real global runner: %v", err)
	}
	if suite.Execution.Command == local.Command || suite.RunnerComposition.Execution.PassSignal != fixture.request.Task.Environment.Commands[0].PassSignal {
		t.Fatalf("local selection command displaced authoritative grading command/pass signal: %+v", suite.RunnerComposition)
	}
}

func TestTestIRGlobalRunnerCompositionBlocked(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	binding := fakeSuiteBinding(t, fixture)
	binding.RunnerComposition.Events = nil
	var err error
	binding.RunnerComposition.Digest, err = semanticir.RunnerCompositionDigest(binding.RunnerComposition)
	if err != nil {
		t.Fatal(err)
	}
	binding.RunnerComposition.Derivation.DecodedModelDigest = binding.RunnerComposition.Digest
	_, err = testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: binding})
	if err == nil || !strings.Contains(err.Error(), "omit grading tests") {
		t.Fatalf("global runner missing the real grading test was not blocked: %v", err)
	}
}

func TestTestIRIndependent(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "first-zero", materializeExact)
	before := compileFixtureStatic(t, fixture)
	for index := range fixture.request.Task.Requirements {
		fixture.request.Task.Requirements[index].RequiredOutcomes = []string{"one"}
		fixture.request.Task.Requirements[index].ForbiddenOutcomes = []string{"zero"}
	}
	after := compileFixtureStatic(t, fixture)
	if !equalTestIRPredicates(before.Predicate, after.Predicate) {
		t.Fatal("Test IR changed when only Spec outcomes changed; verifier translation is not independent")
	}

	// Conversely, changing the supplied TestModel without changing the
	// compiler-derived graph must be rejected rather than trusted.
	model := fixture.request.TestModels[0]
	model.Tests[0].Predicate = semanticir.TestPredicate{Kind: semanticir.PredicateTrue, Provenance: model.Tests[0].Provenance}
	fixture.request.TestModels[0] = model
	replaceAttachedTestArtifact(fixture.request.Task, model)
	_, err := testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: fakeSuiteBinding(t, fixture)})
	if err == nil || !strings.Contains(err.Error(), "compiler-derived") {
		t.Fatalf("unproved frontend predicate was not rejected by graph derivation: %v", err)
	}
}

func TestTestIRPassSignal(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	suite := compileFixtureStatic(t, fixture)
	want := fixture.request.Task.Environment.Commands[0].PassSignal
	if suite.Execution.PassSignal != want {
		t.Fatalf("suite pass signal=%+v, want exact frozen %+v", suite.Execution.PassSignal, want)
	}
	binding := fakeSuiteBinding(t, fixture)
	binding.Execution.PassSignal.Expected = "17"
	_, err := testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: binding})
	if err == nil {
		t.Fatal("Test IR accepted a pass signal different from the translated runner and frozen environment")
	}
}

func TestTestIRMissingTranslationBlocked(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	model := fixture.request.TestModels[0]
	model.Coverage.Status = semanticir.TranslationBlocked
	model.Coverage.Unsupported = []semanticir.UnsupportedConstruct{{Kind: "dynamic-eval", Reason: "reachable verifier behavior is not translated", Provenance: model.Coverage.Provenance}}
	fixture.request.TestModels[0] = model
	replaceAttachedTestArtifact(fixture.request.Task, model)
	_, err := testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: fakeSuiteBinding(t, fixture)})
	if err == nil || !strings.Contains(err.Error(), "not a complete translation") {
		t.Fatalf("incomplete real verifier translation did not block: %v", err)
	}
}

func TestTestIRRunnerSelectionBlocked(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	binding := fakeSuiteBinding(t, fixture)
	model := fixture.request.TestModels[0]
	model.RunnerSelection = nil
	fixture.request.TestModels[0] = model
	replaceAttachedTestArtifact(fixture.request.Task, model)
	binding.SourceModels = append([]semanticir.ArtifactModel(nil), fixture.request.TestModels...)
	_, err := testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: binding})
	if err == nil || !strings.Contains(err.Error(), "runner") {
		t.Fatalf("missing exact runner selection did not block: %v", err)
	}
}

func TestTestIREnvironmentEvidenceBlocked(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	binding := fakeSuiteBinding(t, fixture)
	fixture.request.Task.Environment = nil
	_, err := testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: binding})
	if err == nil || !strings.Contains(err.Error(), "frozen environment") {
		t.Fatalf("missing frozen environment did not block: %v", err)
	}
}

func TestTestIRCompileStaticNoTestsAsTrue(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "all", materializeExact)
	suite, err := testir.CompileStatic(context.Background(), testir.StaticRequest{
		Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: fakeSuiteBinding(t, fixture),
	})
	if err != nil {
		t.Fatalf("CompileStatic: %v", err)
	}
	if suite.Predicate.Kind != semanticir.PredicateTrue || suite.CrossCheck != nil {
		t.Fatalf("explicit complete no-test translation did not compile to static true: %+v", suite)
	}
}

func TestTestIRCompileStaticRejectsMissingQuotientProof(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	binding := fakeSuiteBinding(t, fixture)
	fixture.request.TestModels[0].TestProjection.Nodes = nil
	fixture.request.TestModels[0].TestProjection.PassRoots = nil
	for index := range fixture.request.Task.Artifacts {
		if fixture.request.Task.Artifacts[index].Kind == semanticir.ArtifactTests {
			fixture.request.Task.Artifacts[index] = fixture.request.TestModels[0]
		}
	}
	_, err := testir.CompileStatic(context.Background(), testir.StaticRequest{Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: binding})
	if err == nil {
		t.Fatal("static Test IR accepted missing observation-completeness proof")
	}
}

func TestTestIRCompileStaticRejectsSampleOnlyCategory(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "first-zero", materializeExact)

	// Change semantic label x=a from the singleton {"a"} to the category
	// x>=0, while the translated test still calls only the concrete witness
	// x=0. That sample is evidence about one behavior point, not the category.
	operation := &fixture.request.Task.Operations[0]
	operation.Inputs = []semanticir.Variable{{Name: "x", Type: semanticir.TypeInteger, Provenance: operation.Provenance}}
	prov := fixture.request.Task.Provenance
	variable := semanticir.Expression{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "x", Provenance: prov}
	zero := semanticir.Literal{Type: semanticir.TypeInteger}
	zeroExpr := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &zero, Provenance: prov}
	category := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpGE, Operands: []semanticir.Expression{variable, zeroExpr}, Provenance: prov}
	fixture.request.Task.Domains[0].Values[0].Groundings = []semanticir.GroundingAxiom{{
		OperationID: "f", Kind: semanticir.GroundingMembership, Membership: &category,
		ConcreteWitness: map[string]semanticir.Literal{"x": zero}, Provenance: prov,
	}}
	fixture.request.Task.Domains[0].Values = fixture.request.Task.Domains[0].Values[:1]
	fixture.request.Task.Domains[0].Values[0].ID = "nonnegative"
	fixture.request.Task.Requirements = fixture.request.Task.Requirements[:1]
	fixture.request.Task.Requirements[0].Conditions = semanticir.Assignment{"x": "nonnegative"}
	fixture.request.Task.Requirements[0].ID = "f-x-nonnegative"
	fixture.request.Task.Requirements[0].GroundingID = semanticir.AssignmentGroundingID("f", fixture.request.Task.Requirements[0].Conditions)
	fixture.request.Task.Groundings = []semanticir.AssignmentGrounding{{
		ID: fixture.request.Task.Requirements[0].GroundingID, OperationID: "f", Conditions: semanticir.Assignment{"x": "nonnegative"},
		Inputs: map[string]semanticir.Literal{"x": zero}, Provenance: prov,
	}}

	point := map[string]semanticir.Literal{"x": zero}
	testModel := fixture.request.TestModels[0]
	setTestPredicateConditionsAndInputs(&testModel.Tests[0].Predicate, "f", semanticir.Assignment{"x": "a"}, semanticir.Assignment{"x": "nonnegative"}, point)
	for index := range testModel.TestProjection.Nodes {
		node := &testModel.TestProjection.Nodes[index]
		if node.Observe != nil && node.Observe.Behavior.OperationID == "f" && node.Observe.Behavior.Conditions["x"] == "a" {
			node.Observe.Behavior.Conditions = semanticir.Assignment{"x": "nonnegative"}
			node.Observe.Behavior.Inputs = cloneTestIRInputs(point)
		}
	}
	for index := range testModel.TestProjection.Dependencies {
		dependency := &testModel.TestProjection.Dependencies[index]
		if dependency.Behavior.OperationID == "f" && dependency.Behavior.Conditions["x"] == "a" {
			dependency.Behavior.Conditions = semanticir.Assignment{"x": "nonnegative"}
			dependency.Behavior.Inputs = cloneTestIRInputs(point)
			dependency.Inputs = cloneTestIRInputs(point)
		}
	}
	// No singleton/finite/universal proof exists: the compiler saw one call.
	testModel.TestProjection.Quantification = nil
	wrongAtOne := semanticir.BehaviorRef{OperationID: "f", Conditions: semanticir.Assignment{"x": "nonnegative"}, Inputs: map[string]semanticir.Literal{"x": {Type: semanticir.TypeInteger, Integer: 1}}}
	testedAtZero := semanticir.BehaviorRef{OperationID: "f", Conditions: semanticir.Assignment{"x": "nonnegative"}, Inputs: point}
	if semanticir.BehaviorRefKey(wrongAtOne) == semanticir.BehaviorRefKey(testedAtZero) {
		t.Fatal("wrong implementation at x=1 collapsed into tested point x=0")
	}
	predicate := semanticir.StaticTestPredicate(testModel.Tests, testModel.TestProjection.Provenance)
	predicateDigest, err := semanticir.Digest(predicate)
	if err != nil {
		t.Fatal(err)
	}
	testModel.TestProjection.PredicateDigest = predicateDigest
	testModel.RunnerSelection.PredicateDigest = predicateDigest
	testsDigest, err := semanticir.Digest(testModel.Tests)
	if err != nil {
		t.Fatal(err)
	}
	testModel.TestProjection.Derivation.DecodedModelDigest = testsDigest
	fixture.request.TestModels[0] = testModel
	for index := range fixture.request.Task.Artifacts {
		if fixture.request.Task.Artifacts[index].Kind == semanticir.ArtifactTests {
			fixture.request.Task.Artifacts[index] = testModel
		}
	}
	fixture.request.Task.Tests = append([]semanticir.TestModel(nil), testModel.Tests...)
	binding := fakeSuiteBinding(t, fixture)

	_, err = testir.CompileStatic(context.Background(), testir.StaticRequest{
		Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: binding,
	})
	if err == nil || !strings.Contains(err.Error(), "no quantification evidence") {
		t.Fatalf("sample-only x=0 observation was not rejected as whole-category x>=0 evidence: %v", err)
	}
}

func TestTestIRFiniteCategoryBlocked(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	task := fixture.request.Task
	prov := task.Provenance
	task.Operations[0].Inputs = []semanticir.Variable{{Name: "x", Type: semanticir.TypeBool, Provenance: prov}}
	truth := semanticir.Literal{Type: semanticir.TypeBool, Bool: true}
	membership := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeBool, Literal: &truth, Provenance: prov}
	boolGrounding := semanticir.GroundingAxiom{
		OperationID: "f", Kind: semanticir.GroundingMembership, Membership: &membership,
		ConcreteWitness: map[string]semanticir.Literal{"x": {Type: semanticir.TypeBool}}, Provenance: prov,
	}
	boolValue := semanticir.DomainValue{ID: "all", Provenance: prov, Groundings: []semanticir.GroundingAxiom{boolGrounding}}
	task.Domains = []semanticir.Domain{{ID: "x", Type: semanticir.TypeBool, Provenance: prov, Values: []semanticir.DomainValue{boolValue}}}
	task.Requirements = []semanticir.RequirementCase{{
		ID: "f-all", OperationID: "f", Conditions: semanticir.Assignment{"x": "all"},
		RequiredOutcomes: []string{"zero"}, ForbiddenOutcomes: []string{"one"}, Provenance: prov,
	}}
	task.Groundings = nil
	for artifactIndex := range task.Artifacts {
		artifact := &task.Artifacts[artifactIndex]
		if artifact.Kind != semanticir.ArtifactCode || len(artifact.CompilerEvidence) == 0 || artifact.CompilerEvidence[0].SemanticGraph == nil {
			continue
		}
		for nodeIndex := range artifact.CompilerEvidence[0].SemanticGraph.Nodes {
			node := &artifact.CompilerEvidence[0].SemanticGraph.Nodes[nodeIndex]
			if node.Kind == semanticir.CompilerNodeInput && node.InputName == "x" {
				node.Type = semanticir.TypeBool
			}
		}
	}
	falseInputs := map[string]semanticir.Literal{"x": {Type: semanticir.TypeBool}}
	trueInputs := map[string]semanticir.Literal{"x": {Type: semanticir.TypeBool, Bool: true}}
	testModel := fixture.request.TestModels[0]
	setTestPredicateConditionsAndInputs(&testModel.Tests[0].Predicate, "f", semanticir.Assignment{"x": "a"}, semanticir.Assignment{"x": "all"}, falseInputs)
	setTestPredicateConditionsAndInputs(&testModel.Tests[0].Predicate, "f", semanticir.Assignment{"x": "b"}, semanticir.Assignment{"x": "all"}, trueInputs)
	for nodeIndex := range testModel.TestProjection.Nodes {
		node := &testModel.TestProjection.Nodes[nodeIndex]
		for _, reference := range []*semanticir.BehaviorRef{node.Left, node.Right} {
			if reference == nil || reference.OperationID != "f" {
				continue
			}
			switch reference.Conditions["x"] {
			case "a":
				reference.Conditions = semanticir.Assignment{"x": "all"}
				reference.Inputs = cloneTestIRInputs(falseInputs)
			case "b":
				reference.Conditions = semanticir.Assignment{"x": "all"}
				reference.Inputs = cloneTestIRInputs(trueInputs)
			}
		}
	}
	for dependencyIndex := range testModel.TestProjection.Dependencies {
		dependency := &testModel.TestProjection.Dependencies[dependencyIndex]
		switch dependency.Behavior.Conditions["x"] {
		case "a":
			dependency.Behavior.Conditions = semanticir.Assignment{"x": "all"}
			dependency.Behavior.Inputs = cloneTestIRInputs(falseInputs)
			dependency.Inputs = cloneTestIRInputs(falseInputs)
		case "b":
			dependency.Behavior.Conditions = semanticir.Assignment{"x": "all"}
			dependency.Behavior.Inputs = cloneTestIRInputs(trueInputs)
			dependency.Inputs = cloneTestIRInputs(trueInputs)
		}
	}
	var codeGraph *semanticir.CompilerSemanticGraph
	for _, artifact := range task.Artifacts {
		if artifact.Kind == semanticir.ArtifactCode && len(artifact.CompilerEvidence) != 0 {
			codeGraph = artifact.CompilerEvidence[0].SemanticGraph
		}
	}
	codeGraphDigest, err := semanticir.CompilerSemanticGraphDigest(codeGraph)
	if err != nil {
		t.Fatal(err)
	}
	concreteInputs := []map[string]semanticir.Literal{falseInputs, trueInputs}
	concreteDigest, err := semanticir.TestConcreteInputsDigest(concreteInputs)
	if err != nil {
		t.Fatal(err)
	}
	testModel.TestProjection.Quantification = []semanticir.TestQuantificationEvidence{{
		Behavior: semanticir.BehaviorRef{OperationID: "f", Conditions: semanticir.Assignment{"x": "all"}, Provenance: testModel.Tests[0].Provenance},
		Kind:     semanticir.TestQuantificationFiniteExhaustive, ConcreteInputs: concreteInputs, ConcreteInputsDigest: concreteDigest,
		CodeGraphDigest: codeGraphDigest, Result: semanticir.ProofProved, Provenance: testModel.Tests[0].Provenance,
	}}
	predicate := semanticir.StaticTestPredicate(testModel.Tests, testModel.TestProjection.Provenance)
	predicateDigest, err := semanticir.Digest(predicate)
	if err != nil {
		t.Fatal(err)
	}
	testModel.TestProjection.PredicateDigest = predicateDigest
	testModel.RunnerSelection.PredicateDigest = predicateDigest
	testsDigest, err := semanticir.Digest(testModel.Tests)
	if err != nil {
		t.Fatal(err)
	}
	testModel.TestProjection.Derivation.DecodedModelDigest = testsDigest
	fixture.request.TestModels[0] = testModel
	for index := range task.Artifacts {
		if task.Artifacts[index].Kind == semanticir.ArtifactTests {
			task.Artifacts[index] = testModel
		}
	}
	task.Tests = append([]semanticir.TestModel(nil), testModel.Tests...)

	points, diagnostics := semanticir.ConcreteBehaviorPoints(task)
	if !semanticir.HasErrors(diagnostics) || len(points) != 0 {
		t.Fatalf("non-exact category was silently expanded from samples: points=%+v diagnostics=%+v", points, diagnostics)
	}
	binding := fakeSuiteBinding(t, fixture)
	_, err = testir.CompileStatic(context.Background(), testir.StaticRequest{Task: task, TestModels: fixture.request.TestModels, Binding: binding})
	if err == nil || !strings.Contains(err.Error(), "closed finite input/state set") {
		t.Fatalf("non-exact finite category did not block Test IR: %v", err)
	}
}

func fakeSuiteBinding(t *testing.T, fixture testIRFixture) testir.SuiteBinding {
	t.Helper()
	testModel := fixture.request.TestModels[0]
	testArtifact := testModel.Artifact
	environmentArtifact := fixture.request.Task.Environment.Artifact
	configurationArtifact := testModel.RunnerSelection.Configuration
	testProvenance := testModel.Tests[0].Provenance
	environmentProvenance := fixture.request.Task.Environment.Provenance
	configurationProvenance := testModel.RunnerSelection.Provenance
	verifier := fixture.request.Task.Environment.Commands[0].Tools[0]
	execution := fixture.request.Task.Environment.Commands[0]
	sources := []semanticir.ArtifactRef{testArtifact, environmentArtifact, configurationArtifact}
	binding := testir.SuiteBinding{
		SourceArtifacts: sources, SourceModels: []semanticir.ArtifactModel{testModel}, Verifier: verifier, Execution: execution,
		Provenance: testProvenance, Evidence: []semanticir.Provenance{testProvenance, environmentProvenance, configurationProvenance},
	}
	binding.RunnerComposition = fakeRunnerComposition(t, binding)
	return binding
}

func fakeRunnerComposition(t *testing.T, binding testir.SuiteBinding) semanticir.RunnerCompositionEvidence {
	t.Helper()
	components := make([]semanticir.RunnerCompositionComponent, 0, len(binding.SourceModels))
	var events []semanticir.RunnerEvent
	for _, model := range binding.SourceModels {
		modelDigest, err := semanticir.ArtifactModelTranslationDigest(model)
		if err != nil {
			t.Fatal(err)
		}
		selectionDigest, err := semanticir.RunnerSelectionDigest(*model.RunnerSelection)
		if err != nil {
			t.Fatal(err)
		}
		testIDs := make([]string, len(model.Tests))
		for index, test := range model.Tests {
			testIDs[index] = test.ID
			events = append(events, semanticir.RunnerEvent{
				Ordinal: len(events), ID: "runner-event-" + test.ID, Kind: semanticir.RunnerEventTest,
				ArtifactID: model.Artifact.ID, TestID: test.ID, CompilerNodeIDs: []string{"runner-node-" + test.ID}, Provenance: test.Provenance,
			})
		}
		sort.Strings(testIDs)
		components = append(components, semanticir.RunnerCompositionComponent{
			ArtifactID: model.Artifact.ID, ArtifactDigest: model.Artifact.Digest, ModelDigest: modelDigest,
			SelectionDigest: selectionDigest, TestIDs: testIDs,
		})
	}
	predicate := semanticir.StaticTestPredicate(append([]semanticir.TestModel(nil), binding.SourceModels[0].Tests...), binding.Provenance)
	if len(binding.SourceModels) > 1 {
		var tests []semanticir.TestModel
		for _, model := range binding.SourceModels {
			tests = append(tests, model.Tests...)
		}
		predicate = semanticir.StaticTestPredicate(tests, binding.Provenance)
	}
	predicateDigest, err := semanticir.Digest(predicate)
	if err != nil {
		t.Fatal(err)
	}
	record := semanticir.RunnerCompositionEvidence{
		Kind: semanticir.RunnerCompositionConjunction, SourceArtifacts: append([]semanticir.ArtifactRef(nil), binding.SourceArtifacts...),
		Components: components, Events: events, PredicateDigest: predicateDigest, Verifier: binding.Verifier,
		Execution: binding.Execution, Complete: true, Provenance: binding.Provenance,
	}
	record.Digest, err = semanticir.RunnerCompositionDigest(record)
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest, err := semanticir.TestSuiteSourceDigest(record.SourceArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	empty := []byte{}
	emptyEnvironment := []semanticir.EnvironmentVariable{}
	emptyEnvironmentDigest, _ := semanticir.Digest(emptyEnvironment)
	step := semanticir.ProbeStep{
		ID: "decode-global-runner", Kind: semanticir.ProbeStepRun, Tool: binding.Verifier,
		StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: binding.Execution.WorkingDirectory,
		Environment: emptyEnvironment, EnvironmentDigest: emptyEnvironmentDigest, ClearEnvironment: true, KillProcessGroup: true,
		TimeoutMillis: binding.Execution.TimeoutMillis, ExpectedExitCode: 0, ExpectedStdoutDigest: semanticir.DigestBytes(empty),
		ExpectedStderrDigest: semanticir.DigestBytes(nil), ExpectedSignalDigest: semanticir.DigestBytes([]byte("0")),
		SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: binding.Provenance,
	}
	record.Derivation = semanticir.CompilerDerivationEvidence{
		SourceDigest: sourceDigest, WorkspaceTreeDigest: binding.Execution.TreeDigest, Tool: binding.Verifier,
		IRKind: semanticir.CompilerIRVerifierGraph, IRDigest: semanticir.DigestBytes([]byte("fake-global-runner-ir")),
		Steps: []semanticir.ProbeStep{step}, Output: empty, OutputDigest: semanticir.DigestBytes(empty),
		DecodedModelDigest: record.Digest, Complete: true,
	}
	return record
}

func makeFakeOrderedStatefulRunner(t *testing.T, binding *testir.SuiteBinding) {
	t.Helper()
	record := binding.RunnerComposition
	record.Kind = semanticir.RunnerCompositionOrderedStateful
	record.States = []semanticir.RunnerStateObject{{
		ID: "shared-fixture-state", CompilerNodeIDs: []string{"runner-state-node"}, Provenance: binding.SourceModels[0].Tests[0].Provenance,
	}}
	for index := range record.Events {
		record.Events[index].ReadsStateIDs = []string{"shared-fixture-state"}
		record.Events[index].WritesStateIDs = []string{"shared-fixture-state"}
	}
	var err error
	record.Digest, err = semanticir.RunnerCompositionDigest(record)
	if err != nil {
		t.Fatal(err)
	}
	record.Derivation.DecodedModelDigest = record.Digest
	binding.RunnerComposition = record
}

func compileFixtureStatic(t *testing.T, fixture testIRFixture) semanticir.TestSuiteModel {
	t.Helper()
	suite, err := testir.CompileStatic(context.Background(), testir.StaticRequest{
		Task: fixture.request.Task, TestModels: fixture.request.TestModels, Binding: fakeSuiteBinding(t, fixture),
	})
	if err != nil {
		t.Fatalf("CompileStatic: %v", err)
	}
	return suite
}

func replaceAttachedTestArtifact(task *semanticir.Task, model semanticir.ArtifactModel) {
	for index := range task.Artifacts {
		if task.Artifacts[index].Kind == semanticir.ArtifactTests && task.Artifacts[index].Artifact.ID == model.Artifact.ID {
			task.Artifacts[index] = model
		}
	}
	for index := range task.Coverage {
		if task.Coverage[index].Provenance.ArtifactID == model.Artifact.ID {
			task.Coverage[index] = model.Coverage
		}
	}
	task.Tests = append([]semanticir.TestModel(nil), model.Tests...)
}

func replaceFixtureTestModel(t *testing.T, fixture *testIRFixture, model semanticir.ArtifactModel) {
	t.Helper()
	completeFakeTestEvidence(t, fixture.root, &model, fixture.request.Task.CodeCases,
		fixture.request.Task.Environment.Commands[0], fixture.request.Task.Environment.Tools[0])
	fixture.request.TestModels = []semanticir.ArtifactModel{model}
	replaceAttachedTestArtifact(fixture.request.Task, model)
}

func equalTestIRPredicates(left, right semanticir.TestPredicate) bool {
	leftDigest, leftErr := semanticir.Digest(left)
	rightDigest, rightErr := semanticir.Digest(right)
	return leftErr == nil && rightErr == nil && leftDigest == rightDigest
}

func TestTestIRResourceLimit(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	fixture.request.MaxVectors = 3
	result := testir.Build(context.Background(), fixture.request)
	if result.Status != testir.StatusNotRun || result.NotRun == nil || result.NotRun.Code != "resource-bound" {
		t.Fatalf("resource-bound cross-check status=%s evidence=%+v", result.Status, result.NotRun)
	}
	if err := testir.ValidateEvidence(result); err != nil {
		t.Fatalf("invalid resource-bound evidence: %v", err)
	}
	if len(result.Vectors) != 0 || !result.Execution.Baseline.StartedAt.IsZero() {
		t.Fatal("resource-limited result contains partial vectors or execution evidence")
	}
}

func TestTestIRNoSampling(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	fixture.request.MaxVectors = 3
	result := testir.CrossCheck(context.Background(), fixture.request)
	if result.Status != testir.StatusNotRun || result.NotRun == nil || result.NotRun.TotalVectors != 4 ||
		result.NotRun.VectorCeiling != 3 || len(result.Vectors) != 0 || result.AcceptedVectors != 0 {
		t.Fatalf("resource ceiling produced a partial/sample truth table: %+v", result)
	}
	if err := testir.ValidateEvidence(result); err != nil {
		t.Fatalf("resource-bound evidence is invalid: %v", err)
	}
}

func TestTestIREvidence(t *testing.T) {
	fixture := newSingleArtifactTestIRFixture(t, "equal", materializeExact)
	result := testir.Build(context.Background(), fixture.request)
	assertCompleteTestIR(t, result, 4, 2)
	if err := testir.ValidateEvidence(result); err != nil {
		t.Fatalf("fresh exhaustive evidence rejected: %v", err)
	}
	for _, vector := range result.Vectors {
		if !semanticir.ValidDigest(vector.CandidateSHA256) || !semanticir.ValidDigest(vector.EvidenceSHA256) || len(vector.Retranslations) != 1 {
			t.Fatalf("vector %s lacks immutable candidate/retranslation evidence", vector.ID)
		}
		if len(vector.Retranslations[0].Model.CompilerEvidence) == 0 || len(vector.Retranslations[0].CategoryProofSHA256) == 0 {
			t.Fatalf("vector %s omits its whole-category proof records", vector.ID)
		}
		if len(vector.Command.Command) == 0 || vector.Command.WorkDir == "" || vector.Command.StartedAt.IsZero() || vector.Command.ExitCode == nil {
			t.Fatalf("vector %s lacks exact command/result evidence: %+v", vector.ID, vector.Command)
		}
	}
	result.Vectors[0].TestsPass = !result.Vectors[0].TestsPass
	if err := testir.ValidateEvidence(result); err == nil {
		t.Fatal("mutated vector evidence was accepted")
	}
}

func newSingleArtifactTestIRFixture(t *testing.T, verifier string, mode fakeMaterializeMode) testIRFixture {
	t.Helper()
	root := canonicalTestIRRoot(t)
	operations := []semanticir.Operation{{
		ID: "f", Kind: semanticir.OperationFunction, DomainIDs: []string{"x"}, OutcomeIDs: []string{"zero", "one"},
		Inputs: []semanticir.Variable{{Name: "x", Type: semanticir.TypeString}},
	}}
	task := fakeTask(operations, []semanticir.Domain{{ID: "x", Type: semanticir.TypeString, Values: []semanticir.DomainValue{{ID: "a"}, {ID: "b"}}}}, []semanticir.ObservableOutcome{{ID: "zero", Kind: semanticir.OutcomeReturn}, {ID: "one", Kind: semanticir.OutcomeReturn}})
	testModel := fakeStaticTestModel(t, root, verifier, operations)
	source := []byte("f|x=a|zero\nf|x=b|zero\n")
	binding := fakeBinding(t, root, "impl", "impl.txt", source, operations, task, mode)
	task.CodeCases = append([]semanticir.BehaviorCase(nil), binding.Model.Cases...)
	attachFakeProofEnvironment(task, binding)
	command := verifierCommand(verifier, "impl.txt", "")
	attachFakeSuiteEnvironment(t, task, binding.Frontend.Workspace, command, root)
	completeFakeTestEvidence(t, root, &testModel, task.CodeCases, task.Environment.Commands[0], task.Environment.Tools[0])
	task.Artifacts = []semanticir.ArtifactModel{binding.Model, testModel}
	task.Coverage = []semanticir.TranslationCoverage{binding.Model.Coverage, testModel.Coverage}
	task.Tests = append([]semanticir.TestModel(nil), testModel.Tests...)
	return testIRFixture{root: root, request: testir.Request{
		Task: task, Artifacts: []testir.ArtifactBinding{binding}, TestModels: []semanticir.ArtifactModel{testModel},
		Executor: executor.TaskEnvironment{Command: command, WorkspaceRoot: root, WorkspaceSHA256: binding.Frontend.Workspace.TreeDigest, WorkDir: root, Timeout: 5 * time.Second, ExactEnvironment: true, PassSignal: executor.ExitCodeSignal(0)},
	}}
}

func newMultiArtifactTestIRFixture(t *testing.T) testIRFixture {
	t.Helper()
	root := canonicalTestIRRoot(t)
	operations := []semanticir.Operation{
		{ID: "left", Kind: semanticir.OperationFunction, DomainIDs: []string{"left-input"}, OutcomeIDs: []string{"zero", "one"}, Inputs: []semanticir.Variable{{Name: "left-input", Type: semanticir.TypeString}}},
		{ID: "right", Kind: semanticir.OperationFunction, DomainIDs: []string{"right-input"}, OutcomeIDs: []string{"zero", "one"}, Inputs: []semanticir.Variable{{Name: "right-input", Type: semanticir.TypeString}}},
	}
	task := fakeTask(operations, []semanticir.Domain{
		{ID: "left-input", Type: semanticir.TypeString, Values: []semanticir.DomainValue{{ID: "a"}, {ID: "b"}}},
		{ID: "right-input", Type: semanticir.TypeString, Values: []semanticir.DomainValue{{ID: "a"}, {ID: "b"}}},
	}, []semanticir.ObservableOutcome{{ID: "zero", Kind: semanticir.OutcomeReturn}, {ID: "one", Kind: semanticir.OutcomeReturn}})
	testModel := fakeStaticTestModel(t, root, "multi-equal", operations)
	left := fakeBinding(t, root, "left-code", "left.txt", []byte("left|left-input=a|zero\nleft|left-input=b|zero\n"), operations[:1], task, materializeExact)
	right := fakeBinding(t, root, "right-code", "right.txt", []byte("right|right-input=a|zero\nright|right-input=b|zero\n"), operations[1:], task, materializeExact)
	// Each frontend needs the complete shared workspace context even though it
	// owns only one focus artifact.
	entries := append([]semanticir.WorkspaceEntry{}, left.Frontend.Workspace.Entries...)
	entries = append(entries, right.Frontend.Workspace.Entries...)
	left.Frontend.Workspace.Entries, right.Frontend.Workspace.Entries = entries, entries
	left.Frontend.Workspace.TreeDigest, right.Frontend.Workspace.TreeDigest = fakeWorkspaceTreeDigest(t, root), fakeWorkspaceTreeDigest(t, root)
	// Rebuild the frozen models against the exact shared tree context.
	var diagnostics []semanticir.Diagnostic
	left.Model, diagnostics = left.Translate(context.Background(), left.Frontend)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("shared left reference translation failed: %+v", diagnostics)
	}
	right.Model, diagnostics = right.Translate(context.Background(), right.Frontend)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("shared right reference translation failed: %+v", diagnostics)
	}
	task.CodeCases = append(append([]semanticir.BehaviorCase{}, left.Model.Cases...), right.Model.Cases...)
	attachFakeProofEnvironment(task, left, right)
	multiCommand := verifierCommand("equal", "left.txt", "right.txt")
	attachFakeSuiteEnvironment(t, task, left.Frontend.Workspace, multiCommand, root)
	completeFakeTestEvidence(t, root, &testModel, task.CodeCases, task.Environment.Commands[0], task.Environment.Tools[0])
	task.Artifacts = []semanticir.ArtifactModel{left.Model, right.Model, testModel}
	task.Coverage = []semanticir.TranslationCoverage{left.Model.Coverage, right.Model.Coverage, testModel.Coverage}
	task.Tests = append([]semanticir.TestModel(nil), testModel.Tests...)
	return testIRFixture{root: root, request: testir.Request{
		Task: task, Artifacts: []testir.ArtifactBinding{left, right}, TestModels: []semanticir.ArtifactModel{testModel},
		Executor: executor.TaskEnvironment{
			Command: multiCommand, WorkspaceRoot: root, WorkspaceSHA256: left.Frontend.Workspace.TreeDigest, WorkDir: root,
			Timeout: 5 * time.Second, ExactEnvironment: true, PassSignal: executor.ExitCodeSignal(0),
		},
	}}
}

func fakeStaticTestModel(t *testing.T, root, kind string, operations []semanticir.Operation) semanticir.ArtifactModel {
	t.Helper()
	content := []byte("independently translated fake test: " + kind + "\n")
	path := "tests.fake"
	if err := os.WriteFile(filepath.Join(root, path), content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "runner.fake"), []byte("frozen fake runner selection\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := semanticir.ArtifactRef{ID: "tests", Kind: semanticir.ArtifactTests, Path: path, Digest: semanticir.DigestBytes(content)}
	modelProv := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: path, StartLine: 1, StartColumn: 1}, semanticir.TranslationComplete)
	prov := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	behavior := func(operationID string, conditions semanticir.Assignment) semanticir.BehaviorRef {
		inputs := map[string]semanticir.Literal{}
		for _, operation := range operations {
			if operation.ID != operationID {
				continue
			}
			for _, input := range operation.Inputs {
				label, exists := conditions[input.Name]
				if !exists || input.Type != semanticir.TypeString {
					t.Fatalf("fake test cannot ground input %s/%s from conditions %v", operationID, input.Name, conditions)
				}
				inputs[input.Name] = semanticir.Literal{Type: semanticir.TypeString, String: label}
			}
		}
		return semanticir.BehaviorRef{OperationID: operationID, Conditions: conditions, Inputs: inputs, Provenance: prov}
	}
	outcomeIn := func(ref semanticir.BehaviorRef, ids ...string) semanticir.TestPredicate {
		observation := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: ref, OutcomeIDs: ids, Provenance: prov}
		return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observation, Provenance: prov}
	}
	equal := func(left, right semanticir.BehaviorRef) semanticir.TestPredicate {
		return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeEqual, Left: &left, Right: &right, Provenance: prov}
	}
	var predicate semanticir.TestPredicate
	switch kind {
	case "equal":
		predicate = equal(behavior("f", semanticir.Assignment{"x": "a"}), behavior("f", semanticir.Assignment{"x": "b"}))
	case "first-zero":
		predicate = outcomeIn(behavior("f", semanticir.Assignment{"x": "a"}), "zero")
	case "all":
		predicate = semanticir.TestPredicate{Kind: semanticir.PredicateTrue, Provenance: prov}
	case "multi-equal":
		predicate = semanticir.TestPredicate{Kind: semanticir.PredicateAnd, Provenance: prov, Children: []semanticir.TestPredicate{
			equal(behavior("left", semanticir.Assignment{"left-input": "a"}), behavior("right", semanticir.Assignment{"right-input": "a"})),
			equal(behavior("left", semanticir.Assignment{"left-input": "b"}), behavior("right", semanticir.Assignment{"right-input": "b"})),
		}}
	default:
		t.Fatalf("unknown fake static test kind %q", kind)
	}
	return semanticir.ArtifactModel{
		Artifact: artifact, Language: semanticir.LanguagePython, Kind: semanticir.ArtifactTests,
		Operations: []semanticir.Operation{{ID: "test-" + kind, Kind: semanticir.OperationTest, Provenance: prov}},
		Tests:      []semanticir.TestModel{{ID: "test-" + kind, Predicate: predicate, Provenance: prov}},
		Coverage:   semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 2, TranslatedConstructs: 2, Provenance: modelProv},
		Translator: fakeToolRef(t, "/bin/sh", "fake-test-translator"),
	}
}

func completeFakeTestEvidence(t *testing.T, root string, model *semanticir.ArtifactModel, _ []semanticir.BehaviorCase, command semanticir.WorkspaceCommand, _ semanticir.ToolRef) {
	t.Helper()
	prov := model.Tests[0].Provenance
	predicate := semanticir.StaticTestPredicate(model.Tests, prov)
	predicateDigest, err := semanticir.Digest(predicate)
	if err != nil {
		t.Fatal(err)
	}
	testsDigest, err := semanticir.Digest(model.Tests)
	if err != nil {
		t.Fatal(err)
	}
	irDigest := semanticir.DigestBytes([]byte("fake-test-compiler-ir:" + model.Artifact.Digest))
	constructs := []semanticir.TestConstructEvidence{
		{ID: "fake-test-call", ArtifactID: model.Artifact.ID, Kind: semanticir.TestConstructCall, Digest: semanticir.DigestBytes([]byte("fake-test-call")), IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: irDigest, Tool: model.Translator, CompilerNodeIDs: []string{"test-call"}, Provenance: prov},
		{ID: "fake-test-assertion", ArtifactID: model.Artifact.ID, Kind: semanticir.TestConstructAssertion, Digest: semanticir.DigestBytes([]byte("fake-test-assertion")), IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: irDigest, Tool: model.Translator, CompilerNodeIDs: []string{"test-assertion"}, Provenance: prov},
	}
	refs := testPredicateBehaviors(predicate)
	dependencies := make([]semanticir.TestBehaviorDependency, 0, len(constructs)*len(refs))
	seenRefs := map[string]bool{}
	for _, ref := range refs {
		key := ref.OperationID + "\x00" + testIRAssignmentText(ref.Conditions)
		if seenRefs[key] {
			continue
		}
		seenRefs[key] = true
		for _, construct := range constructs {
			dependencies = append(dependencies, semanticir.TestBehaviorDependency{ConstructID: construct.ID, Kind: semanticir.TestDependencyCall,
				Behavior: ref, Inputs: cloneTestIRInputs(ref.Inputs), CompilerNodeIDs: append([]string(nil), construct.CompilerNodeIDs...), Provenance: prov})
		}
	}
	quantification := make([]semanticir.TestQuantificationEvidence, 0, len(seenRefs))
	seenCategories := map[string]bool{}
	for _, ref := range refs {
		category := ref
		category.Inputs = nil
		key := category.OperationID + "\x00" + testIRAssignmentText(category.Conditions)
		if seenCategories[key] {
			continue
		}
		seenCategories[key] = true
		concrete := []map[string]semanticir.Literal{cloneTestIRInputs(ref.Inputs)}
		digest, digestErr := semanticir.TestConcreteInputsDigest(concrete)
		if digestErr != nil {
			t.Fatal(digestErr)
		}
		quantification = append(quantification, semanticir.TestQuantificationEvidence{
			Behavior: category, Kind: semanticir.TestQuantificationSingleton, ConcreteInputs: concrete,
			ConcreteInputsDigest: digest, Result: semanticir.ProofProved, Provenance: prov,
		})
	}
	probeTool := fakeToolRef(t, "/usr/bin/true", "fake-test-ir-probe")
	emptyEnvironment := []semanticir.EnvironmentVariable{}
	emptyEnvironmentDigest, _ := semanticir.Digest(emptyEnvironment)
	emptyDigest := semanticir.DigestBytes(nil)
	step := semanticir.ProbeStep{ID: "extract-test-ir", Kind: semanticir.ProbeStepRun, Tool: probeTool,
		StdinDigest: emptyDigest, WorkingDirectory: root, Environment: emptyEnvironment, EnvironmentDigest: emptyEnvironmentDigest,
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000, ExpectedExitCode: 0,
		ExpectedStdoutDigest: emptyDigest, ExpectedStderrDigest: emptyDigest, ExpectedSignalDigest: semanticir.DigestBytes([]byte("0")),
		SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: prov}
	output := []byte{}
	nodes, roots := fakeTestProjectionGraphs(model.Tests, constructs)
	testIDs := make([]string, len(model.Tests))
	for index, test := range model.Tests {
		testIDs[index] = test.ID
	}
	model.TestProjection = &semanticir.TestObservationProjection{Source: model.Artifact, TestIDs: testIDs, PredicateDigest: predicateDigest,
		Constructs: constructs, Dependencies: dependencies, Nodes: nodes, PassRoots: roots, Quantification: quantification, Complete: true, Provenance: prov,
		Derivation: semanticir.CompilerDerivationEvidence{SourceDigest: model.Artifact.Digest, WorkspaceTreeDigest: command.TreeDigest,
			Tool: model.Translator, IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: irDigest, Steps: []semanticir.ProbeStep{step},
			Output: output, OutputDigest: semanticir.DigestBytes(output), DecodedModelDigest: testsDigest, Complete: true}}
	configurationContent, err := os.ReadFile(filepath.Join(root, "runner.fake"))
	if err != nil {
		t.Fatal(err)
	}
	configuration := semanticir.ArtifactRef{ID: "runner-config", Kind: semanticir.ArtifactConfiguration, Path: "runner.fake", Digest: semanticir.DigestBytes(configurationContent)}
	runnerProv := semanticir.NewProvenance(configuration, semanticir.SourceLocation{Path: configuration.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	model.RunnerSelection = &semanticir.RunnerSelectionEvidence{TestIDs: append([]string(nil), testIDs...), PredicateDigest: predicateDigest,
		Configuration: configuration, Verifier: command.Tools[0], Command: command, ConjunctivePass: true, Complete: true, Provenance: runnerProv}
}

func fakeTestProjectionGraphs(tests []semanticir.TestModel, constructs []semanticir.TestConstructEvidence) ([]semanticir.TestProjectionNode, []semanticir.TestPassRoot) {
	var nodes []semanticir.TestProjectionNode
	var lower func(semanticir.TestPredicate) string
	lower = func(value semanticir.TestPredicate) string {
		id := fmt.Sprintf("test-predicate-%d", len(nodes))
		index := len(nodes)
		node := semanticir.TestProjectionNode{
			ID: id, Kind: value.Kind, Observe: value.Observe, Left: value.Left, Right: value.Right,
			CompilerNodeIDs: []string{id + "-ir"}, Provenance: value.Provenance,
		}
		if value.Kind != semanticir.PredicateAnd && value.Kind != semanticir.PredicateOr && value.Kind != semanticir.PredicateNot {
			for _, construct := range constructs {
				node.ConstructIDs = append(node.ConstructIDs, construct.ID)
			}
		}
		nodes = append(nodes, node)
		for _, child := range value.Children {
			nodes[index].Children = append(nodes[index].Children, lower(child))
		}
		return id
	}
	var roots []semanticir.TestPassRoot
	for _, test := range tests {
		root := lower(test.Predicate)
		roots = append(roots, semanticir.TestPassRoot{TestID: test.ID, NodeID: root, CompilerNodeIDs: []string{root + "-root-ir"}})
	}
	return nodes, roots
}

func testPredicateBehaviors(predicate semanticir.TestPredicate) []semanticir.BehaviorRef {
	var refs []semanticir.BehaviorRef
	if predicate.Observe != nil {
		refs = append(refs, predicate.Observe.Behavior)
	}
	if predicate.Left != nil {
		refs = append(refs, *predicate.Left)
	}
	if predicate.Right != nil {
		refs = append(refs, *predicate.Right)
	}
	for _, child := range predicate.Children {
		refs = append(refs, testPredicateBehaviors(child)...)
	}
	return refs
}

func setTestPredicateConditionsAndInputs(predicate *semanticir.TestPredicate, operationID string, oldConditions, conditions semanticir.Assignment, inputs map[string]semanticir.Literal) {
	if predicate == nil {
		return
	}
	if predicate.Observe != nil && predicate.Observe.Behavior.OperationID == operationID && testIRAssignmentText(predicate.Observe.Behavior.Conditions) == testIRAssignmentText(oldConditions) {
		predicate.Observe.Behavior.Conditions = cloneTestIRAssignment(conditions)
		predicate.Observe.Behavior.Inputs = cloneTestIRInputs(inputs)
	}
	for _, reference := range []*semanticir.BehaviorRef{predicate.Left, predicate.Right} {
		if reference != nil && reference.OperationID == operationID && testIRAssignmentText(reference.Conditions) == testIRAssignmentText(oldConditions) {
			reference.Conditions = cloneTestIRAssignment(conditions)
			reference.Inputs = cloneTestIRInputs(inputs)
		}
	}
	for index := range predicate.Children {
		setTestPredicateConditionsAndInputs(&predicate.Children[index], operationID, oldConditions, conditions, inputs)
	}
}

func cloneTestIRInputs(inputs map[string]semanticir.Literal) map[string]semanticir.Literal {
	if inputs == nil {
		return nil
	}
	copy := make(map[string]semanticir.Literal, len(inputs))
	for name, literal := range inputs {
		copy[name] = literal
	}
	return copy
}

func canonicalTestIRRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func fakeTask(operations []semanticir.Operation, domains []semanticir.Domain, outcomes []semanticir.ObservableOutcome) *semanticir.Task {
	prov := fakeProvenance("spec", "spec.md", []byte("spec"))
	for operationIndex := range operations {
		for inputIndex := range operations[operationIndex].Inputs {
			operations[operationIndex].Inputs[inputIndex].Provenance = prov
		}
		operations[operationIndex].Provenance = prov
	}
	for domainIndex := range domains {
		domain := &domains[domainIndex]
		domain.Provenance = prov
		for valueIndex := range domain.Values {
			value := &domain.Values[valueIndex]
			value.Provenance = prov
			for _, operation := range operations {
				usesDomain := false
				for _, domainID := range operation.DomainIDs {
					usesDomain = usesDomain || domainID == domain.ID
				}
				if !usesDomain {
					continue
				}
				literal := semanticir.Literal{Type: domain.Type, String: value.ID}
				variable := semanticir.Expression{Kind: semanticir.ExprVariable, Type: domain.Type, Name: domain.ID, Provenance: prov}
				literalExpression := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: domain.Type, Literal: &literal, Provenance: prov}
				membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{variable, literalExpression}, Provenance: prov}
				value.Groundings = append(value.Groundings, semanticir.GroundingAxiom{
					OperationID: operation.ID, Kind: semanticir.GroundingMembership, Membership: &membership,
					ConcreteWitness: map[string]semanticir.Literal{domain.ID: literal}, Provenance: prov,
				})
			}
		}
	}
	task := &semanticir.Task{
		ID: "test-ir-task", Spec: semanticir.ArtifactRef{ID: "spec", Kind: semanticir.ArtifactSpec, Path: "spec.md", Digest: semanticir.DigestBytes([]byte("spec"))},
		Instruction: semanticir.ArtifactRef{ID: "instruction", Kind: semanticir.ArtifactInstruction, Path: "instruction.md", Digest: semanticir.DigestBytes([]byte("instruction"))},
		Domains:     domains, Operations: operations, Outcomes: outcomes, Provenance: prov,
		Environment: &semanticir.EnvironmentModel{Provenance: fakeProvenance("environment", "ray.toml", []byte("environment"))},
	}
	for _, operation := range operations {
		assignments := []semanticir.Assignment{{}}
		for _, domainID := range operation.DomainIDs {
			var domain semanticir.Domain
			for _, candidate := range domains {
				if candidate.ID == domainID {
					domain = candidate
				}
			}
			var next []semanticir.Assignment
			for _, assignment := range assignments {
				for _, value := range domain.Values {
					copy := cloneTestIRAssignment(assignment)
					copy[domainID] = value.ID
					next = append(next, copy)
				}
			}
			assignments = next
		}
		for _, assignment := range assignments {
			inputs, singleton := semanticir.ExactGroundingInputs(operation, domains, assignment)
			if !singleton {
				continue
			}
			groundingID := semanticir.AssignmentGroundingID(operation.ID, assignment)
			task.Groundings = append(task.Groundings, semanticir.AssignmentGrounding{
				ID: groundingID, OperationID: operation.ID, Conditions: cloneTestIRAssignment(assignment),
				Inputs: cloneTestIRInputs(inputs), Provenance: prov,
			})
			task.Requirements = append(task.Requirements, semanticir.RequirementCase{
				ID: operation.ID + "-" + testIRAssignmentKey(assignment), OperationID: operation.ID, Conditions: assignment,
				RequiredOutcomes: []string{"zero"}, ForbiddenOutcomes: []string{"one"}, GroundingID: groundingID, Provenance: prov,
			})
		}
	}
	return task
}

func fakeOwnedGroundings(values []semanticir.AssignmentGrounding, operations []semanticir.Operation) []semanticir.AssignmentGrounding {
	owned := map[string]bool{}
	for _, operation := range operations {
		owned[operation.ID] = true
	}
	var result []semanticir.AssignmentGrounding
	for _, value := range values {
		if owned[value.OperationID] {
			result = append(result, value)
		}
	}
	return result
}

func fakeBinding(t *testing.T, root, artifactID, path string, source []byte, owned []semanticir.Operation, options ...any) testir.ArtifactBinding {
	t.Helper()
	scope := &semanticir.Task{}
	mode := materializeExact
	switch len(options) {
	case 1: // compatibility for certificate fixtures that replace the scope after construction
		var ok bool
		mode, ok = options[0].(fakeMaterializeMode)
		if !ok {
			t.Fatalf("fake binding option is %T, want fakeMaterializeMode", options[0])
		}
	case 2:
		var ok bool
		scope, ok = options[0].(*semanticir.Task)
		if !ok || scope == nil {
			t.Fatalf("fake binding scope is %T, want *semanticir.Task", options[0])
		}
		mode, ok = options[1].(fakeMaterializeMode)
		if !ok {
			t.Fatalf("fake binding mode is %T, want fakeMaterializeMode", options[1])
		}
	default:
		t.Fatalf("fake binding received %d options, want mode or scope+mode", len(options))
	}
	if err := os.WriteFile(filepath.Join(root, path), source, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := semanticir.ArtifactRef{ID: artifactID, Kind: semanticir.ArtifactCode, Path: path, Digest: semanticir.DigestBytes(source)}
	prov := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: path, StartLine: 1, StartColumn: 1}, semanticir.TranslationComplete)
	workspace := semanticir.WorkspaceRef{
		ID: "workspace", State: semanticir.WorkspaceSolutionNewTests, Root: root, WorkingDirectory: ".", BuildCommand: "true",
		Entries: []semanticir.WorkspaceEntry{{Path: path, Artifact: artifact, Provenance: prov}}, Provenance: prov,
	}
	workspace.TreeDigest = fakeWorkspaceTreeDigest(t, root)
	prover := fakePinnedProver(t)
	changedRange := semanticir.ChangedSourceRange{
		ArtifactID: artifact.ID, Path: artifact.Path, StartLine: 1, EndLine: len(strings.Split(strings.TrimSuffix(string(source), "\n"), "\n")),
		SliceDigest: semanticir.DigestBytes(source), Provenance: prov,
	}
	frontend := semanticir.FrontendRequest{
		TaskID: "test-ir-task", Artifact: artifact, Kind: semanticir.ArtifactCode, Language: semanticir.LanguagePython,
		Source: source, FiniteDomains: append([]semanticir.Domain(nil), scope.Domains...), Constraints: append([]semanticir.Constraint(nil), scope.Constraints...),
		Operations: owned, Groundings: fakeOwnedGroundings(scope.Groundings, owned), Translator: prover,
		Prover: prover, Workspace: workspace, FocusArtifacts: []semanticir.ArtifactRef{artifact}, ChangedRanges: []semanticir.ChangedSourceRange{changedRange},
	}
	translate := fakeTranslator(root, owned, prover)
	model, diagnostics := translate(context.Background(), frontend)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("fake reference translation failed: %+v", diagnostics)
	}
	return testir.ArtifactBinding{Frontend: frontend, Model: model, Translate: translate, Materialize: fakeMaterializer(owned, mode)}
}

func fakeWorkspaceTreeDigest(t *testing.T, root string) string {
	t.Helper()
	digest, err := executor.WorkspaceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func fakeTranslator(proofRoot string, owned []semanticir.Operation, prover semanticir.ToolRef) testir.TranslateFunc {
	return func(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic) {
		modelProv := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationComplete)
		factProv := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
		model := semanticir.ArtifactModel{
			Artifact: request.Artifact, Language: semanticir.LanguagePython, Kind: semanticir.ArtifactCode,
			Domains: append([]semanticir.Domain(nil), request.FiniteDomains...), Constraints: append([]semanticir.Constraint(nil), request.Constraints...),
			Outcomes: append([]semanticir.ObservableOutcome(nil), request.Outcomes...), Groundings: append([]semanticir.AssignmentGrounding(nil), request.Groundings...),
			Translator: request.Translator,
			Coverage: semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 1, TranslatedConstructs: 1,
				Provenance: modelProv},
		}
		ownedSet := map[string]bool{}
		for _, operation := range owned {
			ownedSet[operation.ID] = true
			copy := operation
			copy.Inputs = append([]semanticir.Variable(nil), operation.Inputs...)
			copy.Provenance = factProv
			for index := range copy.Inputs {
				copy.Inputs[index].Provenance = factProv
			}
			model.Operations = append(model.Operations, copy)
		}
		for lineNumber, line := range strings.Split(strings.TrimSpace(string(request.Source)), "\n") {
			parts := strings.Split(line, "|")
			if len(parts) != 3 || !ownedSet[parts[0]] {
				return model, []semanticir.Diagnostic{{Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported, Message: "invalid fake candidate source"}}
			}
			assignment := semanticir.Assignment{}
			if parts[1] != "" {
				for _, item := range strings.Split(parts[1], ",") {
					key, value, ok := strings.Cut(item, "=")
					if !ok {
						return model, []semanticir.Diagnostic{{Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported, Message: "invalid assignment"}}
					}
					assignment[key] = value
				}
			}
			caseProv := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: lineNumber + 1, StartColumn: 1}, semanticir.TranslationTranslated)
			inputs := make(map[string]semanticir.Literal, len(assignment))
			for name, value := range assignment {
				inputs[name] = semanticir.Literal{Type: semanticir.TypeString, String: value}
			}
			model.Cases = append(model.Cases, semanticir.BehaviorCase{ID: fmt.Sprintf("case-%d", lineNumber), OperationID: parts[0], Conditions: assignment, Inputs: inputs, OutcomeIDs: []string{parts[2]}, Provenance: caseProv})
		}
		evidence, err := fakeCompilerEvidence(ctx, proofRoot, request, model, prover)
		if err != nil {
			return model, []semanticir.Diagnostic{{Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported, Message: err.Error()}}
		}
		model.CompilerEvidence = []semanticir.CompilerEvidence{evidence}
		model.ScopeClosure, err = fakeScopeClosure(ctx, proofRoot, request, model, prover)
		if err != nil {
			return model, []semanticir.Diagnostic{{Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported, Message: err.Error()}}
		}
		return model, nil
	}
}

func fakeMaterializer(owned []semanticir.Operation, mode fakeMaterializeMode) testir.MaterializeFunc {
	return func(_ context.Context, request semanticir.MaterializationRequest) (semanticir.EditPlan, []semanticir.Diagnostic) {
		ownedSet := map[string]bool{}
		for _, operation := range owned {
			ownedSet[operation.ID] = true
		}
		var lines []string
		for _, choice := range request.Counterexample.Choices {
			if !ownedSet[choice.Behavior.OperationID] {
				continue
			}
			outcome := choice.OutcomeID
			if mode == materializeIncorrect && choice.OutcomeID == "one" {
				outcome = "zero"
				if len(choice.Behavior.Conditions) != 0 && choice.Behavior.Conditions["x"] == "b" {
					outcome = "one"
				}
			}
			lines = append(lines, choice.Behavior.OperationID+"|"+testIRAssignmentText(choice.Behavior.Conditions)+"|"+outcome)
		}
		sort.Strings(lines)
		replacement := []byte(strings.Join(lines, "\n") + "\n")
		if mode == materializeNoop {
			replacement = append([]byte(nil), request.Frontend.Source...)
		}
		artifact := request.Frontend.Artifact
		if mode == materializeStale {
			artifact.Digest = semanticir.DigestBytes([]byte("stale"))
		}
		prov := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
		return semanticir.EditPlan{
			ID: request.Counterexample.ID + ":" + artifact.ID, WitnessID: request.Counterexample.ID, Artifact: artifact,
			Edits:      []semanticir.ByteRangeReplacement{{StartByte: 0, EndByte: len(request.Frontend.Source), ExpectedBytes: append([]byte(nil), request.Frontend.Source...), Replacement: replacement}},
			Provenance: prov,
		}, nil
	}
}

func attachFakeProofEnvironment(task *semanticir.Task, bindings ...testir.ArtifactBinding) {
	if len(bindings) == 0 || len(bindings[0].Model.CompilerEvidence) == 0 {
		return
	}
	prover := bindings[0].Model.CompilerEvidence[0].Prover
	environmentArtifact := semanticir.ArtifactRef{
		ID: "environment", Kind: semanticir.ArtifactEnvironment, Path: "ray.toml",
		Digest: semanticir.DigestBytes([]byte("environment")),
	}
	proofEnvironment := []semanticir.EnvironmentVariable{}
	proofEnvironmentDigest, _ := semanticir.Digest(proofEnvironment)
	task.Environment.Artifact = environmentArtifact
	task.Environment.Identity = "fake-hermetic-environment"
	task.Environment.ConfigDigest = proofEnvironmentDigest
	task.Environment.Tools = []semanticir.ToolRef{prover}
	task.Environment.Coverage = semanticir.TranslationCoverage{
		Status: semanticir.TranslationComplete, TotalConstructs: 1, TranslatedConstructs: 1,
		Provenance: semanticir.NewProvenance(environmentArtifact, semanticir.SourceLocation{Path: environmentArtifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationComplete),
	}
	task.Environment.Provenance = task.Environment.Coverage.Provenance
}

func attachFakeSuiteEnvironment(t *testing.T, task *semanticir.Task, workspace semanticir.WorkspaceRef, command []string, root string) {
	t.Helper()
	toolRoot := t.TempDir()
	toolPath := filepath.Join(toolRoot, "fake-verifier")
	toolBytes, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(toolPath, toolBytes, 0o700); err != nil {
		t.Fatal(err)
	}
	verifier := fakeToolRef(t, toolPath, "shell-verifier")
	task.Environment.Tools = append(task.Environment.Tools, verifier)
	exactEnvironment := []semanticir.EnvironmentVariable{}
	exactEnvironmentDigest, err := semanticir.Digest(exactEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	provenance := task.Environment.Provenance
	task.Environment.Commands = append(task.Environment.Commands, semanticir.WorkspaceCommand{
		ID: "solution-new-tests", WorkspaceID: workspace.ID, State: semanticir.WorkspaceSolutionNewTests,
		TreeDigest: workspace.TreeDigest, Command: command[2], WorkingDirectory: root,
		Environment: exactEnvironment, EnvironmentDigest: exactEnvironmentDigest, ClearEnvironment: true, KillProcessGroup: true,
		TimeoutMillis: 5000, PassSignal: semanticir.PassSignal{Kind: semanticir.PassSignalExitCode, Expected: "0", Provenance: provenance},
		ExpectedPass: true, ObservedPass: true, ExitCode: 0,
		StdoutDigest: semanticir.DigestBytes(nil), StderrDigest: semanticir.DigestBytes(nil), SignalValueDigest: semanticir.DigestBytes([]byte("0")),
		Tools: []semanticir.ToolRef{verifier}, Provenance: provenance,
	})
}

func fakePinnedProver(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath("z3")
	if err != nil {
		t.Fatalf("z3 is required for executable Test IR proof tests: %v", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	versionBytes, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("z3 --version: %v: %s", err, versionBytes)
	}
	return semanticir.ToolRef{
		Name: "z3", Path: path, Digest: semanticir.DigestBytes(body), Version: strings.TrimSpace(string(versionBytes)),
	}
}

func fakeToolRef(t *testing.T, path, name string) semanticir.ToolRef {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatal(err)
	}
	return semanticir.ToolRef{Name: name, Path: resolved, Digest: semanticir.DigestBytes(body), Version: "test-pinned"}
}

func fakeCompilerEvidence(ctx context.Context, root string, request semanticir.FrontendRequest, model semanticir.ArtifactModel, prover semanticir.ToolRef) (semanticir.CompilerEvidence, error) {
	provenance := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	maxDomains := 0
	for _, operation := range model.Operations {
		if len(operation.DomainIDs) > maxDomains {
			maxDomains = len(operation.DomainIDs)
		}
	}
	var declarationLines []string
	for index := 0; index < maxDomains; index++ {
		declarationLines = append(declarationLines, fmt.Sprintf("(declare-const d_%d Int)", index))
	}
	declarationLines = append(declarationLines, "(declare-const out Int)")
	declarations := []byte(strings.Join(declarationLines, "\n"))
	emittedIRDigest := semanticir.DigestBytes(declarations)
	harnessFormula := []byte("true")
	harnessDigest := semanticir.DigestBytes(harnessFormula)
	proofContext := semanticir.CompilerProofContext{
		SourceDigest: request.Artifact.Digest, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		EmittedIRDigest: emittedIRDigest, HarnessDigest: harnessDigest, Compiler: request.Translator,
	}
	predicate := func(formula string, node string) semanticir.CompilerPredicate {
		formulaBytes := []byte(formula)
		return semanticir.CompilerPredicate{
			Logic: semanticir.ProofLogicSMTLIB2, Declarations: declarations,
			DeclarationsDigest: semanticir.DigestBytes(declarations), Formula: formulaBytes,
			FormulaDigest: semanticir.DigestBytes(formulaBytes), Tool: request.Translator,
			IRDigest: emittedIRDigest, CompilerNodeIDs: []string{node},
		}
	}
	type partitionKey struct{ operationID, domainID string }
	labelsByPartition := map[partitionKey]map[string]semanticir.LabelPathEvidence{}
	var partitions []semanticir.DomainPartitionEvidence
	var operationScopes []semanticir.OperationScopeEvidence
	for _, operation := range model.Operations {
		for domainIndex, domainID := range operation.DomainIDs {
			valueSet := map[string]bool{}
			for _, behaviorCase := range model.Cases {
				if behaviorCase.OperationID == operation.ID {
					valueSet[behaviorCase.Conditions[domainID]] = true
				}
			}
			valueIDs := make([]string, 0, len(valueSet))
			for valueID := range valueSet {
				if valueID == "" {
					return semanticir.CompilerEvidence{}, fmt.Errorf("fake proof cannot infer an empty domain value for %s/%s", operation.ID, domainID)
				}
				valueIDs = append(valueIDs, valueID)
			}
			sort.Strings(valueIDs)
			if len(valueIDs) == 0 {
				return semanticir.CompilerEvidence{}, fmt.Errorf("fake proof cannot infer domain %s/%s", operation.ID, domainID)
			}
			variable := fmt.Sprintf("d_%d", domainIndex)
			// All predicates in one proof claim must share exact declarations.
			makePredicate := func(formula string, node string) semanticir.CompilerPredicate {
				formulaBytes := []byte(formula)
				return semanticir.CompilerPredicate{
					Logic: semanticir.ProofLogicSMTLIB2, Declarations: declarations,
					DeclarationsDigest: semanticir.DigestBytes(declarations), Formula: formulaBytes,
					FormulaDigest: semanticir.DigestBytes(formulaBytes), Tool: request.Translator,
					IRDigest: emittedIRDigest, CompilerNodeIDs: []string{node},
				}
			}
			scope := makePredicate(fmt.Sprintf("(and (<= 0 %s) (< %s %d))", variable, variable, len(valueIDs)), "scope-"+operation.ID+"-"+domainID)
			if domainIndex == 0 {
				operationScopes = append(operationScopes, semanticir.OperationScopeEvidence{
					OperationID: operation.ID, ScopePredicateDigest: scope.FormulaDigest,
					ScopePredicate: scope, Provenance: provenance,
				})
			}
			labels := make([]semanticir.LabelPathEvidence, 0, len(valueIDs))
			labelMap := map[string]semanticir.LabelPathEvidence{}
			memberships := make([]semanticir.CompilerPredicate, 0, len(valueIDs))
			for valueIndex, valueID := range valueIDs {
				membership := makePredicate(fmt.Sprintf("(= %s %d)", variable, valueIndex), "label-"+operation.ID+"-"+domainID+"-"+valueID)
				claim := semanticir.NewProofClaim(semanticir.ClaimReachability, proofContext, scope, []semanticir.CompilerPredicate{membership}, nil)
				reachability, err := fakeReplayProof(ctx, root, prover, claim, semanticir.SolverSAT)
				if err != nil {
					return semanticir.CompilerEvidence{}, err
				}
				literal := semanticir.Literal{Type: semanticir.TypeString, String: valueID}
				witnessDigest, _ := semanticir.Digest(literal)
				label := semanticir.LabelPathEvidence{
					ValueID: valueID, PredicateDigest: membership.FormulaDigest, MembershipPredicate: membership,
					CompilerNodeIDs: append([]string(nil), membership.CompilerNodeIDs...), Reachability: semanticir.ProofProved,
					ReachabilityProofDigest: reachability.QueryDigest, ReachabilityProof: reachability,
					ConcreteWitness: &literal, WitnessDigest: witnessDigest, Provenance: provenance,
				}
				labels = append(labels, label)
				labelMap[valueID] = label
				memberships = append(memberships, membership)
			}
			totality, err := fakeReplayProof(ctx, root, prover, semanticir.NewProofClaim(semanticir.ClaimTotality, proofContext, scope, memberships, nil), semanticir.SolverUNSAT)
			if err != nil {
				return semanticir.CompilerEvidence{}, err
			}
			disjointness, err := fakeReplayProof(ctx, root, prover, semanticir.NewProofClaim(semanticir.ClaimDisjointness, proofContext, scope, memberships, nil), semanticir.SolverUNSAT)
			if err != nil {
				return semanticir.CompilerEvidence{}, err
			}
			partitions = append(partitions, semanticir.DomainPartitionEvidence{
				OperationID: operation.ID, DomainID: domainID, ScopePredicateDigest: scope.FormulaDigest, ScopePredicate: scope,
				Labels: labels, Totality: semanticir.ProofProved, TotalityProofDigest: totality.QueryDigest, TotalityProof: totality,
				Disjointness: semanticir.ProofProved, DisjointnessProofDigest: disjointness.QueryDigest, DisjointnessProof: disjointness,
				Provenance: provenance,
			})
			labelsByPartition[partitionKey{operation.ID, domainID}] = labelMap
		}
	}

	var behaviorProofs []semanticir.BehaviorRealizationEvidence
	for _, behaviorCase := range model.Cases {
		var memberships []semanticir.CompilerPredicate
		var predicateDigests []string
		for _, operation := range model.Operations {
			if operation.ID != behaviorCase.OperationID {
				continue
			}
			for _, domainID := range operation.DomainIDs {
				label := labelsByPartition[partitionKey{operation.ID, domainID}][behaviorCase.Conditions[domainID]]
				memberships = append(memberships, label.MembershipPredicate)
				predicateDigests = append(predicateDigests, label.PredicateDigest)
			}
		}
		if len(memberships) == 0 {
			return semanticir.CompilerEvidence{}, fmt.Errorf("fake proof cannot encode zero-domain behavior case %s", behaviorCase.ID)
		}
		behaviorPredicate := predicate(string(harnessFormula), "behavior-"+behaviorCase.ID)
		outcomePredicates := make([]semanticir.CompilerOutcomePredicate, 0, len(behaviorCase.OutcomeIDs))
		for _, outcomeID := range behaviorCase.OutcomeIDs {
			outcomePredicates = append(outcomePredicates, semanticir.CompilerOutcomePredicate{OutcomeID: outcomeID, Predicate: behaviorPredicate})
		}
		var scope semanticir.CompilerPredicate
		for _, partition := range partitions {
			if partition.OperationID == behaviorCase.OperationID {
				scope = partition.ScopePredicate
				break
			}
		}
		claim := semanticir.NewProofClaim(semanticir.ClaimRealization, proofContext, scope, memberships, outcomePredicates)
		realization, err := fakeReplayProof(ctx, root, prover, claim, semanticir.SolverUNSAT)
		if err != nil {
			return semanticir.CompilerEvidence{}, err
		}
		behaviorProofs = append(behaviorProofs, semanticir.BehaviorRealizationEvidence{
			BehaviorCaseID: behaviorCase.ID,
			Behavior:       semanticir.BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: cloneTestIRAssignment(behaviorCase.Conditions), Inputs: cloneTestIRInputs(behaviorCase.Inputs), Provenance: behaviorCase.Provenance},
			OutcomeIDs:     append([]string(nil), behaviorCase.OutcomeIDs...), CategoryPredicateDigests: predicateDigests,
			RealizationProof: realization, Provenance: behaviorCase.Provenance,
		})
	}
	var outcomeClosures []semanticir.OutcomeClosureEvidence
	for _, operation := range model.Operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		var scope semanticir.CompilerPredicate
		for _, candidate := range operationScopes {
			if candidate.OperationID == operation.ID {
				scope = candidate.ScopePredicate
				break
			}
		}
		var declared []semanticir.CompilerOutcomePredicate
		var memberships []semanticir.CompilerPredicate
		var complements []string
		other := semanticir.OtherOutcome(operation.ID, provenance)
		for index, outcomeID := range operation.OutcomeIDs {
			if outcomeID == other.ID {
				continue
			}
			formula := fmt.Sprintf("(= out %d)", index)
			membership := predicate(formula, "outcome-"+operation.ID+"-"+outcomeID)
			declared = append(declared, semanticir.CompilerOutcomePredicate{OutcomeID: outcomeID, Predicate: membership})
			memberships = append(memberships, membership)
			complements = append(complements, "(not "+formula+")")
		}
		otherFormula := "true"
		if len(complements) != 0 {
			otherFormula = "(and " + strings.Join(complements, " ") + ")"
		}
		otherPredicate := predicate(otherFormula, "outcome-"+operation.ID+"-other")
		memberships = append(memberships, otherPredicate)
		totality, err := fakeReplayProof(ctx, root, prover, semanticir.NewProofClaim(semanticir.ClaimTotality, proofContext, scope, memberships, nil), semanticir.SolverUNSAT)
		if err != nil {
			return semanticir.CompilerEvidence{}, err
		}
		disjointness, err := fakeReplayProof(ctx, root, prover, semanticir.NewProofClaim(semanticir.ClaimDisjointness, proofContext, scope, memberships, nil), semanticir.SolverUNSAT)
		if err != nil {
			return semanticir.CompilerEvidence{}, err
		}
		boundaryDigest, err := semanticir.Digest(struct {
			Declared   []semanticir.CompilerOutcomePredicate
			Complement semanticir.CompilerPredicate
		}{declared, otherPredicate})
		if err != nil {
			return semanticir.CompilerEvidence{}, err
		}
		outcomeClosures = append(outcomeClosures, semanticir.OutcomeClosureEvidence{
			OperationID: operation.ID, BoundaryDigest: boundaryDigest, Declared: declared,
			Complements: []semanticir.OutcomeComplement{{ID: other.ID, Kind: semanticir.OutcomeComplementReturn,
				Description: "all other observable outcomes", Predicate: semanticir.CompilerOutcomePredicate{OutcomeID: other.ID, Predicate: otherPredicate}}},
			Totality: semanticir.ProofProved, TotalityProof: totality,
			Disjointness: semanticir.ProofProved, DisjointnessProof: disjointness, Provenance: provenance,
		})
	}
	emptyEnvironment := []semanticir.EnvironmentVariable{}
	emptyEnvironmentDigest, err := semanticir.Digest(emptyEnvironment)
	if err != nil {
		return semanticir.CompilerEvidence{}, err
	}
	var graphNodes []semanticir.CompilerSemanticNode
	var graphBlocks []semanticir.CompilerSemanticBlock
	var graphOperations []semanticir.CompilerOperationGraph
	var graphConstructs []semanticir.CompilerConstructBinding
	for _, operation := range model.Operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		constructID := "fake-graph-" + operation.ID
		nodeID := "fake-success-" + operation.ID
		blockID := "fake-block-" + operation.ID
		var blockNodeIDs []string
		var inputBindings []semanticir.CompilerInputNode
		for _, input := range operation.Inputs {
			inputNodeID := "fake-input-" + operation.ID + "-" + input.Name
			graphNodes = append(graphNodes, semanticir.CompilerSemanticNode{
				ID: inputNodeID, Kind: semanticir.CompilerNodeInput, Type: input.Type, InputName: input.Name,
				CompilerNodeIDs: []string{constructID}, Provenance: provenance,
			})
			blockNodeIDs = append(blockNodeIDs, inputNodeID)
			inputBindings = append(inputBindings, semanticir.CompilerInputNode{InputName: input.Name, NodeID: inputNodeID})
		}
		graphNodes = append(graphNodes, semanticir.CompilerSemanticNode{
			ID: nodeID, Kind: semanticir.CompilerNodeSuccess, Type: semanticir.TypeUnit,
			CompilerNodeIDs: []string{constructID}, Provenance: provenance,
		})
		blockNodeIDs = append(blockNodeIDs, nodeID)
		graphBlocks = append(graphBlocks, semanticir.CompilerSemanticBlock{
			ID: blockID, NodeIDs: blockNodeIDs, CompilerNodeIDs: []string{constructID}, Provenance: provenance,
		})
		graphOperations = append(graphOperations, semanticir.CompilerOperationGraph{
			OperationID: operation.ID, EntryBlockID: blockID, Inputs: inputBindings, TerminalNodeIDs: []string{nodeID}, Provenance: provenance,
		})
		graphConstructs = append(graphConstructs, semanticir.CompilerConstructBinding{
			ID: constructID, Kind: semanticir.CompilerConstructReturn, Opcode: "success",
			SemanticNodeIDs: append([]string(nil), blockNodeIDs...), BlockIDs: []string{blockID}, Provenance: provenance,
		})
	}
	probeStep := func(id string, stdoutDigest string) semanticir.ProbeStep {
		return semanticir.ProbeStep{
			ID: id, Kind: semanticir.ProbeStepRun, Tool: request.Translator, Argv: []string{"--version"},
			StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: root,
			Environment: emptyEnvironment, EnvironmentDigest: emptyEnvironmentDigest,
			ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000,
			ExpectedExitCode: 0, ExpectedStdoutDigest: stdoutDigest, ExpectedStderrDigest: semanticir.DigestBytes(nil),
			ExpectedSignalDigest: semanticir.DigestBytes(nil), SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone},
			Provenance: provenance,
		}
	}
	semanticGraph := &semanticir.CompilerSemanticGraph{
		SourceDigest: request.Artifact.Digest, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		Tool: request.Translator, IRKind: semanticir.CompilerIRCPythonBytecode, IR: declarations, IRDigest: emittedIRDigest,
		Environment: emptyEnvironment, EnvironmentDigest: emptyEnvironmentDigest,
		DerivationSteps: []semanticir.ProbeStep{probeStep("fake-compile-ir", emittedIRDigest)},
		Nodes:           graphNodes, Blocks: graphBlocks, Operations: graphOperations, Constructs: graphConstructs, Provenance: provenance,
	}
	decoderOutput, err := semanticir.CanonicalCompilerDecoderOutput(semanticGraph)
	if err != nil {
		return semanticir.CompilerEvidence{}, err
	}
	semanticGraph.DecoderSteps = []semanticir.ProbeStep{probeStep("fake-decode-ir", semanticir.DigestBytes(decoderOutput))}
	semanticGraph.DecoderOutput = decoderOutput
	semanticGraph.DecoderOutputDigest = semanticir.DigestBytes(decoderOutput)
	graphDigest, err := semanticir.CompilerSemanticGraphDigest(semanticGraph)
	if err != nil {
		return semanticir.CompilerEvidence{}, err
	}
	return semanticir.CompilerEvidence{
		ID: "fake-compiler-" + request.Artifact.ID, Method: semanticir.CompilerEvidenceModelChecker,
		FormulaDerivationDigest: graphDigest,
		Tool:                    request.Translator, Prover: prover,
		SourceDigest: request.Artifact.Digest, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		Argv: []string{request.Translator.Path, request.Artifact.Path}, EnvironmentDigest: emptyEnvironmentDigest,
		IRKind: semanticir.CompilerIRCPythonBytecode, EmittedIRDigest: emittedIRDigest, HarnessDigest: harnessDigest,
		TotalConstructs: len(graphConstructs), TranslatedConstructs: len(graphConstructs), Partitions: partitions, BehaviorProofs: behaviorProofs, Provenance: provenance,
		OperationScopes: operationScopes, OutcomeClosures: outcomeClosures,
		SemanticGraph: semanticGraph,
	}, nil
}

func fakeScopeClosure(ctx context.Context, root string, request semanticir.FrontendRequest, model semanticir.ArtifactModel, prover semanticir.ToolRef) (*semanticir.ScopeClosureEvidence, error) {
	prov := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	declaration := semanticir.CompilerDeclaration{
		ID: "decl-" + request.Artifact.ID, QualifiedName: "fake::" + request.Artifact.ID, Artifact: request.Artifact,
		Location:        semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1, EndLine: len(strings.Split(strings.TrimSuffix(string(request.Source), "\n"), "\n"))},
		CompilerNodeIDs: []string{"decl-node-" + request.Artifact.ID}, Changed: true, Provenance: prov,
	}
	owners := make([]semanticir.OperationOwner, 0, len(model.Operations))
	for _, operation := range model.Operations {
		if operation.Kind != semanticir.OperationTest {
			owners = append(owners, semanticir.OperationOwner{OperationID: operation.ID, DeclarationID: declaration.ID})
		}
	}
	changedRanges := append([]semanticir.ChangedSourceRange(nil), request.ChangedRanges...)
	for index := range changedRanges {
		changedRanges[index].ArtifactID = request.Artifact.ID
		changedRanges[index].Path = request.Artifact.Path
		changedRanges[index].StartLine = 1
		changedRanges[index].EndLine = len(strings.Split(strings.TrimSuffix(string(request.Source), "\n"), "\n"))
		changedRanges[index].SliceDigest = semanticir.DigestBytes(request.Source)
		changedRanges[index].Provenance = prov
	}
	evidence := &semanticir.ScopeClosureEvidence{
		SourceArtifacts: []semanticir.ArtifactRef{request.Artifact}, WorkspaceTreeDigest: request.Workspace.TreeDigest,
		Compiler: request.Translator, Prover: prover, CompilerIRDigest: model.CompilerEvidence[0].EmittedIRDigest,
		ChangedRanges: changedRanges, Declarations: []semanticir.CompilerDeclaration{declaration},
		ImpactedDeclarationIDs: []string{declaration.ID}, OperationOwners: owners, Completeness: semanticir.ProofProved, Complete: true, Provenance: prov,
	}
	graphDigest, err := semanticir.ScopeClosureGraphDigest(*evidence)
	if err != nil {
		return nil, err
	}
	sourceDigest, err := semanticir.Digest(evidence.SourceArtifacts)
	if err != nil {
		return nil, err
	}
	declarations := []byte{}
	// Scope-closure queries ask whether an omitted impacted declaration
	// exists. The complete fake graph has none, so its omission predicate is
	// identically false.
	formula := []byte("false")
	scope := semanticir.CompilerPredicate{
		Logic: semanticir.ProofLogicSMTLIB2, Declarations: declarations, DeclarationsDigest: semanticir.DigestBytes(declarations),
		Formula: formula, FormulaDigest: semanticir.DigestBytes(formula), Tool: request.Translator,
		IRDigest: evidence.CompilerIRDigest, CompilerNodeIDs: []string{"scope-closure"},
	}
	proofContext := semanticir.CompilerProofContext{
		SourceDigest: sourceDigest, WorkspaceTreeDigest: request.Workspace.TreeDigest, EmittedIRDigest: evidence.CompilerIRDigest,
		HarnessDigest: graphDigest, Compiler: request.Translator,
	}
	claim := semanticir.NewProofClaim(semanticir.ClaimScopeClosure, proofContext, scope, nil, nil)
	evidence.CompletenessProof, err = fakeReplayProof(ctx, root, prover, claim, semanticir.SolverUNSAT)
	if err != nil {
		return nil, err
	}
	return evidence, nil
}

func fakeReplayProof(ctx context.Context, root string, prover semanticir.ToolRef, claim semanticir.ProofClaim, expected semanticir.SolverResult) (semanticir.ReplayableProof, error) {
	query, err := semanticir.CanonicalProofQuery(claim)
	if err != nil {
		return semanticir.ReplayableProof{}, err
	}
	proofEnvironment := []semanticir.EnvironmentVariable{}
	environmentDigest, err := semanticir.Digest(proofEnvironment)
	if err != nil {
		return semanticir.ReplayableProof{}, err
	}
	argv := []string{"-in", "-smt2"}
	command := exec.CommandContext(ctx, prover.Path, argv...)
	command.Dir = root
	command.Env = []string{}
	command.Stdin = strings.NewReader(string(query))
	output, err := command.Output()
	if err != nil {
		return semanticir.ReplayableProof{}, fmt.Errorf("execute fake proof: %w", err)
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 || semanticir.SolverResult(fields[0]) != expected {
		return semanticir.ReplayableProof{}, fmt.Errorf("proof returned %q, want %s", strings.TrimSpace(string(output)), expected)
	}
	return semanticir.ReplayableProof{
		Claim: claim, Logic: semanticir.ProofLogicSMTLIB2, Query: query, QueryDigest: semanticir.DigestBytes(query),
		Prover: prover, Argv: argv, WorkingDirectory: root, Environment: proofEnvironment, EnvironmentDigest: environmentDigest,
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000,
		SolverOutput: output, SolverOutputDigest: semanticir.DigestBytes(output), Result: expected,
		SubjectDigests: semanticir.ProofClaimSubjectDigests(claim),
	}, nil
}

func verifierCommand(kind, left, right string) []string {
	script := "exit 0"
	switch kind {
	case "equal":
		if right == "" {
			script = fmt.Sprintf(`a=$(awk -F'|' '$2=="x=a" {print $3}' %s); b=$(awk -F'|' '$2=="x=b" {print $3}' %s); test "$a" = "$b"`, left, left)
		} else {
			script = fmt.Sprintf(`a=$(awk -F'|' '{print $3}' %s); b=$(awk -F'|' '{print $3}' %s); test "$a" = "$b"`, left, right)
		}
	case "first-zero":
		script = fmt.Sprintf(`a=$(awk -F'|' '$2=="x=a" {print $3}' %s); test "$a" = zero`, left)
	case "all":
		script = "exit 0"
	}
	return []string{"/bin/sh", "-c", script}
}

func assertCompleteTestIR(t *testing.T, result testir.Result, total, accepted uint64) {
	t.Helper()
	if result.Status != testir.StatusComplete || len(result.Blockers) != 0 {
		t.Fatalf("Test IR blocked: status=%s blockers=%+v", result.Status, result.Blockers)
	}
	if result.TotalVectors != total || uint64(len(result.Vectors)) != total || result.AcceptedVectors != accepted {
		t.Fatalf("truth table totals got total=%d rows=%d accepted=%d, want %d/%d/%d", result.TotalVectors, len(result.Vectors), result.AcceptedVectors, total, total, accepted)
	}
	if err := testir.ValidateEvidence(result); err != nil {
		t.Fatalf("invalid Test IR evidence: %v", err)
	}
}

func assertBlockedTestIR(t *testing.T, result testir.Result, code string) {
	t.Helper()
	if result.Status != testir.StatusBlocked {
		t.Fatalf("status=%s, want PROOF BLOCKED", result.Status)
	}
	if err := testir.ValidateEvidence(result); err != nil {
		t.Fatalf("blocked Test IR lacks immutable evidence: %v", err)
	}
	for _, blocker := range result.Blockers {
		if blocker.Code == code {
			return
		}
	}
	t.Fatalf("missing blocker %q in %+v", code, result.Blockers)
}

func acceptedVectors(result testir.Result) []string {
	var values []string
	for _, vector := range result.Vectors {
		if vector.TestsPass {
			values = append(values, compactVector(vector.Choices))
		}
	}
	sort.Strings(values)
	return values
}

func compactVector(choices []semanticir.BehaviorChoice) string {
	var builder strings.Builder
	for _, choice := range choices {
		if choice.OutcomeID == "zero" {
			builder.WriteByte('0')
		} else {
			builder.WriteByte('1')
		}
	}
	return builder.String()
}

func assertPredicateTruth(t *testing.T, predicate semanticir.TestPredicate, vectors []testir.VectorEvidence) {
	t.Helper()
	for _, vector := range vectors {
		got := evaluateTestIRPredicate(predicate, vector.Choices)
		if got != vector.TestsPass {
			t.Fatalf("predicate(%s)=%v, executable verifier=%v", compactVector(vector.Choices), got, vector.TestsPass)
		}
	}
}

func evaluateTestIRPredicate(predicate semanticir.TestPredicate, choices []semanticir.BehaviorChoice) bool {
	switch string(predicate.Kind) {
	case "false":
		return false
	case string(semanticir.PredicateTrue):
		return true
	case string(semanticir.PredicateAnd):
		for _, child := range predicate.Children {
			if !evaluateTestIRPredicate(child, choices) {
				return false
			}
		}
		return true
	case string(semanticir.PredicateOr):
		for _, child := range predicate.Children {
			if evaluateTestIRPredicate(child, choices) {
				return true
			}
		}
		return false
	case string(semanticir.PredicateOutcomeIn):
		for _, choice := range choices {
			if choice.Behavior.OperationID == predicate.Observe.Behavior.OperationID && equalTestIRAssignments(choice.Behavior.Conditions, predicate.Observe.Behavior.Conditions) {
				for _, outcome := range predicate.Observe.OutcomeIDs {
					if choice.OutcomeID == outcome {
						return true
					}
				}
			}
		}
		return false
	case string(semanticir.PredicateOutcomeEqual):
		if predicate.Left == nil || predicate.Right == nil {
			return false
		}
		left, leftOK := testIROutcomeFor(*predicate.Left, choices)
		right, rightOK := testIROutcomeFor(*predicate.Right, choices)
		return leftOK && rightOK && left == right
	default:
		panic("unsupported test predicate in fixture: " + predicate.Kind)
	}
}

func testIROutcomeFor(reference semanticir.BehaviorRef, choices []semanticir.BehaviorChoice) (string, bool) {
	for _, choice := range choices {
		if choice.Behavior.OperationID == reference.OperationID && equalTestIRAssignments(choice.Behavior.Conditions, reference.Conditions) {
			return choice.OutcomeID, true
		}
	}
	return "", false
}

func fakeProvenance(id, path string, content []byte) semanticir.Provenance {
	artifact := semanticir.ArtifactRef{ID: id, Kind: semanticir.ArtifactSpec, Path: path, Digest: semanticir.DigestBytes(content)}
	return semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: path, StartLine: 1, StartColumn: 1}, semanticir.TranslationComplete)
}

func cloneTestIRAssignment(value semanticir.Assignment) semanticir.Assignment {
	result := make(semanticir.Assignment, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func equalTestIRAssignments(left, right semanticir.Assignment) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func testIRAssignmentKey(value semanticir.Assignment) string {
	return strings.ReplaceAll(testIRAssignmentText(value), ",", "-")
}

func testIRAssignmentText(value semanticir.Assignment) string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+value[key])
	}
	return strings.Join(parts, ",")
}
