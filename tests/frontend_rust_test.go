package tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/internal/executor"
	frontendrust "github.com/HyperMarble/hyperray/internal/frontend/rust"
	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func TestFrontendRust(t *testing.T) {
	code := `fn helper(x: i32) -> i32 {
    if x >= 10 { 10 } else { x }
}

fn classify(x: i32, enabled: bool) -> Result<i32, &'static str> {
    if !enabled {
        return Err("disabled");
    }
    match x {
        0 => Ok(helper(x)),
        _ if x < 0 => panic!("negative"),
        _ => Ok(helper(x)),
    }
}

fn sum_to(limit: i32) -> i32 {
    let mut total = 0;
    for value in 0..limit {
        total = total + value;
    }
    total
}
`
	domains := []semanticir.Domain{
		rustDomain("helper.x", "-1", "0", "10"),
		rustDomain("classify.x", "-1", "0", "10"),
		rustDomain("classify.enabled", "false", "true"),
		rustDomain("sum_to.limit", "0", "3"),
	}
	request := rustFrontendRequest(t, semanticir.ArtifactCode, code, domains)
	request.EntryPoints = []string{"classify"}
	request.Outcomes = []semanticir.ObservableOutcome{
		rustReturnOutcome(request.Artifact, "helper", -1), rustReturnOutcome(request.Artifact, "helper", 0), rustReturnOutcome(request.Artifact, "helper", 10),
		semanticir.OtherOutcome("helper", semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)),
		rustExceptionalOutcome(request.Artifact, "classify", semanticir.OutcomeSuccess, "", ""),
		rustExceptionalOutcome(request.Artifact, "classify", semanticir.OutcomeRaise, "Result::Err", ""),
		rustExceptionalOutcome(request.Artifact, "classify", semanticir.OutcomeRaise, "panic", "negative"),
		semanticir.OtherOutcome("classify", semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)),
		rustReturnOutcome(request.Artifact, "sum_to", 0), rustReturnOutcome(request.Artifact, "sum_to", 3),
		semanticir.OtherOutcome("sum_to", semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)),
	}
	request.Operations = []semanticir.Operation{
		{ID: "helper", Kind: semanticir.OperationCallable, DomainIDs: []string{"helper.x"}, OutcomeIDs: []string{request.Outcomes[0].ID, request.Outcomes[1].ID, request.Outcomes[2].ID, request.Outcomes[3].ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "x", semanticir.TypeInteger, "helper.x")}},
		{ID: "classify", Kind: semanticir.OperationCallable, DomainIDs: []string{"classify.x", "classify.enabled"}, OutcomeIDs: []string{request.Outcomes[4].ID, request.Outcomes[5].ID, request.Outcomes[6].ID, request.Outcomes[7].ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "x", semanticir.TypeInteger, "classify.x"), rustSpecInput(request.Artifact, "enabled", semanticir.TypeBool, "classify.enabled")}},
		{ID: "sum_to", Kind: semanticir.OperationCallable, DomainIDs: []string{"sum_to.limit"}, OutcomeIDs: []string{request.Outcomes[8].ID, request.Outcomes[9].ID, request.Outcomes[10].ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "limit", semanticir.TypeInteger, "sum_to.limit")}},
	}
	rustFinalizeGroundings(&request)
	model, diagnostics := frontendrust.Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("Translate(code) blocked: %+v", diagnostics)
	}
	if model.Coverage.Status != semanticir.TranslationComplete || model.Coverage.TotalConstructs != model.Coverage.TranslatedConstructs {
		t.Fatalf("coverage is not complete: %+v", model.Coverage)
	}
	if model.Translator != request.Translator {
		t.Fatalf("translator evidence detached: got %+v want %+v", model.Translator, request.Translator)
	}
	if validation := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(validation) {
		t.Fatalf("translated Rust artifact model is outside the frozen request scope: %+v", validation)
	}
	if len(model.CompilerEvidence) != 0 || len(model.ExhaustiveEvidence) != 1 {
		t.Fatalf("Rust scalar semantics are not exclusively backed by exhaustive execution: compiler=%d exhaustive=%d", len(model.CompilerEvidence), len(model.ExhaustiveEvidence))
	}
	exhaustive := model.ExhaustiveEvidence[0]
	if exhaustive.Replay.Clean || len(exhaustive.Replay.Runs) != 0 {
		t.Fatalf("Rust frontend forged caller-owned replay confirmation: %+v", exhaustive.Replay)
	}
	if len(exhaustive.Runs) != 2 || len(exhaustive.Steps) != 1+2*len(model.Cases) || exhaustive.Runs[0].OrderDigest == exhaustive.Runs[1].OrderDigest || exhaustive.Runs[0].ObservationDigest != exhaustive.Runs[1].ObservationDigest {
		t.Fatalf("Rust exhaustive execution is not two complete independent fresh-process orders: %+v", exhaustive)
	}
	for _, run := range exhaustive.Runs {
		if run.FreshProcessCount != len(model.Cases) {
			t.Fatalf("Rust run process count = %d, want %d", run.FreshProcessCount, len(model.Cases))
		}
		for _, observation := range run.Observations {
			if observation.StepID == "" || semanticir.ValidateRawOutcomeTrace(observation.RawOutcome) != nil || observation.StdoutDigest != semanticir.DigestBytes(observation.Stdout) || observation.StderrDigest != semanticir.DigestBytes(observation.Stderr) || observation.SignalValueDigest != semanticir.DigestBytes(observation.SignalValue) {
				t.Fatalf("Rust execution observation is not raw-outcome/step/byte bound: %+v", observation)
			}
			if encoded, err := semanticir.CanonicalJSON(observation.RawOutcome); err != nil || !bytes.Equal(encoded, observation.SignalValue) {
				t.Fatalf("Rust execution signal contains forged semantic identity instead of the raw runtime trace: %+v", observation)
			}
			for _, forbidden := range [][]byte{[]byte(`"id"`), []byte(`"operation_id"`), []byte(`"provenance"`)} {
				if bytes.Contains(observation.SignalValue, forbidden) {
					t.Fatalf("Rust raw trace signal self-reports forbidden semantic identity %s: %s", forbidden, observation.SignalValue)
				}
			}
		}
	}
	replay := executor.ReplayExhaustive(context.Background(), executor.ExhaustiveReplayPlan{
		ID: "rust-frontend-replay",
		Workspace: executor.ProbeWorkspace{
			ID: request.Workspace.ID, Root: request.Workspace.Root, State: request.Workspace.State, TreeSHA256: request.Workspace.TreeDigest,
		},
		SourceArtifacts: []semanticir.ArtifactRef{request.Artifact},
		Operations:      append([]semanticir.Operation(nil), model.Operations...),
		Evidence:        exhaustive,
	})
	if replay.Status != executor.StatusConfirmed || len(replay.Blockers) != 0 {
		t.Fatalf("Rust exhaustive transcript did not replay in fresh workspaces: %+v", replay.Blockers)
	}
	if err := executor.ValidateExhaustiveReplay(replay); err != nil {
		t.Fatalf("Rust exhaustive replay evidence is invalid: %v", err)
	}
	semanticReplay, err := executor.SemanticReplay(replay)
	if err != nil {
		t.Fatalf("Rust exhaustive replay cannot be attached to Semantic IR: %v", err)
	}
	model.ExhaustiveEvidence[0].Replay = semanticReplay
	if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
		t.Fatalf("centrally replayed Rust artifact model is structurally invalid: %+v", validation)
	}
	if model.ScopeClosure == nil || !model.ScopeClosure.Complete || model.ScopeClosure.Completeness != semanticir.ProofProved || len(model.ScopeClosure.Declarations) != 3 || len(model.ScopeClosure.OperationOwners) != 3 {
		t.Fatalf("Rust patch-scope closure is incomplete: %+v", model.ScopeClosure)
	}
	if got := len(model.Cases); got != 11 {
		t.Fatalf("all function behavior cases = %d, want 11", got)
	}
	classify := rustOperation(t, model, "classify")
	if len(classify.DomainIDs) != 2 || len(classify.OutcomeIDs) < 3 {
		t.Fatalf("operation universe not scoped: %+v", classify)
	}
	statementKinds := map[semanticir.StatementKind]bool{}
	expressionKinds := map[semanticir.ExpressionKind]bool{}
	for _, operation := range model.Operations {
		checkRustProvenance(t, operation.Provenance, request.Artifact)
		walkRustStatements(t, operation.Body, request.Artifact, statementKinds, expressionKinds)
	}
	for _, kind := range []semanticir.StatementKind{semanticir.StmtBranch, semanticir.StmtReturn, semanticir.StmtRaise, semanticir.StmtAssign, semanticir.StmtLoop} {
		if !statementKinds[kind] {
			t.Errorf("missing lowered statement kind %q", kind)
		}
	}
	for _, kind := range []semanticir.ExpressionKind{semanticir.ExprCompare, semanticir.ExprCall} {
		if !expressionKinds[kind] {
			t.Errorf("missing lowered expression kind %q", kind)
		}
	}
	seenSuccess, seenResultErr, seenPanic := false, false, false
	for _, outcome := range model.Outcomes {
		checkRustProvenance(t, outcome.Provenance, request.Artifact)
		if outcome.ID != semanticir.OutcomeID(outcome) {
			t.Errorf("non-canonical outcome ID %q", outcome.ID)
		}
		seenSuccess = seenSuccess || outcome.Kind == semanticir.OutcomeSuccess
		seenResultErr = seenResultErr || outcome.Kind == semanticir.OutcomeRaise && outcome.ExceptionType == "Result::Err"
		seenPanic = seenPanic || outcome.Kind == semanticir.OutcomeRaise && outcome.ExceptionType == "panic" && outcome.Message == "negative"
	}
	if !seenSuccess || !seenResultErr || !seenPanic {
		t.Fatalf("missing Result/panic outcomes: %+v", model.Outcomes)
	}

	testSource := code + `
#[test]
fn assertions_are_global() {
    assert_eq!(classify(0, true), Ok(0));
    assert_ne!(classify(0, true), classify(10, true));
    assert!(!is_negative(0));
}

fn is_negative(x: i32) -> bool { x < 0 }
`
	testDomains := append(append([]semanticir.Domain(nil), domains...), rustDomain("is_negative.x", "0"))
	testRequest := rustFrontendRequest(t, semanticir.ArtifactTests, testSource, testDomains)
	falseOutcome := rustBoolOutcome(testRequest.Artifact, "is_negative", false)
	trueOutcome := rustBoolOutcome(testRequest.Artifact, "is_negative", true)
	otherOutcome := semanticir.OtherOutcome("is_negative", semanticir.NewProvenance(testRequest.Artifact, semanticir.SourceLocation{Path: testRequest.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated))
	testRequest.Outcomes = append(append([]semanticir.ObservableOutcome(nil), request.Outcomes...), falseOutcome, trueOutcome, otherOutcome)
	testRequest.Operations = append(append([]semanticir.Operation(nil), request.Operations...), semanticir.Operation{ID: "is_negative", Kind: semanticir.OperationCallable, DomainIDs: []string{"is_negative.x"}, OutcomeIDs: []string{falseOutcome.ID, trueOutcome.ID, otherOutcome.ID}, Inputs: []semanticir.Variable{rustSpecInput(testRequest.Artifact, "x", semanticir.TypeInteger, "is_negative.x")}})
	rustFinalizeGroundings(&testRequest)
	testModel, testDiagnostics := frontendrust.Translate(context.Background(), testRequest)
	if !semanticir.HasErrors(testDiagnostics) || testModel.Coverage.Status != semanticir.TranslationBlocked {
		t.Fatalf("Rust test translation certified without a compiler-derived dependency graph: model=%+v diagnostics=%+v", testModel, testDiagnostics)
	}
	if len(testModel.Tests) != 1 {
		t.Fatalf("test models = %d, want 1; diagnostics=%+v", len(testModel.Tests), testDiagnostics)
	}
	test := testModel.Tests[0]
	if len(test.Assertions) != 3 || test.Predicate.Kind != semanticir.PredicateAnd || len(test.Predicate.Children) != 3 {
		t.Fatalf("assertions/global predicate not lowered: %+v", test)
	}
	if test.Predicate.Children[1].Kind != semanticir.PredicateNot || len(test.Predicate.Children[1].Children) != 1 || test.Predicate.Children[1].Children[0].Kind != semanticir.PredicateOutcomeEqual {
		t.Fatalf("cross-case assert_ne! is not a negated global equality: %+v", test.Predicate.Children[1])
	}
	checkRustProvenance(t, test.Provenance, testRequest.Artifact)
	checkRustProvenance(t, test.Predicate.Provenance, testRequest.Artifact)
	if testModel.TestProjection != nil || testModel.RunnerSelection != nil {
		t.Fatalf("blocked Rust test translation emitted self-claimed projection evidence: projection=%+v runner=%+v", testModel.TestProjection, testModel.RunnerSelection)
	}
}

func TestFrontendRustBlocked(t *testing.T) {
	tests := map[string]struct {
		source      string
		operationID string
		inputName   string
	}{
		"unsafe":           {source: `unsafe fn raw(p: *const i32) -> i32 { *p }`, operationID: "raw", inputName: "p"},
		"ffi":              {source: `extern "C" { fn foreign(x: i32) -> i32; }`, operationID: "foreign", inputName: "x"},
		"unresolved_macro": {source: `fn f() -> i32 { mystery!() }`, operationID: "f"},
		"untranslated":     {source: `fn f(x: i32) -> i32 { let mut y = x; y += 1; y }`, operationID: "f", inputName: "x"},
	}
	for name, fixture := range tests {
		t.Run(name, func(t *testing.T) {
			var domains []semanticir.Domain
			var domainIDs []string
			var inputs []semanticir.Variable
			if fixture.inputName != "" {
				domainID := fixture.operationID + "." + fixture.inputName
				domains = []semanticir.Domain{rustDomain(domainID, "0")}
				domainIDs = []string{domainID}
				inputs = []semanticir.Variable{rustSpecInput(semanticir.ArtifactRef{}, fixture.inputName, semanticir.TypeInteger, domainID)}
			}
			request := rustFrontendRequest(t, semanticir.ArtifactCode, fixture.source, domains)
			for index := range inputs {
				inputs[index].Provenance = semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
			}
			returned := rustReturnOutcome(request.Artifact, fixture.operationID, 0)
			other := semanticir.OtherOutcome(fixture.operationID, semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated))
			request.Outcomes = []semanticir.ObservableOutcome{returned, other}
			request.Operations = []semanticir.Operation{{ID: fixture.operationID, Kind: semanticir.OperationCallable, DomainIDs: domainIDs, OutcomeIDs: []string{returned.ID, other.ID}, Inputs: inputs}}
			rustFinalizeGroundings(&request)
			model, diagnostics := frontendrust.Translate(context.Background(), request)
			if !semanticir.HasErrors(diagnostics) {
				t.Fatalf("unsupported Rust translated without a blocker: model=%+v diagnostics=%+v", model, diagnostics)
			}
			if model.Coverage.Status != semanticir.TranslationBlocked || len(model.Coverage.Unsupported) == 0 {
				t.Fatalf("unsupported Rust did not block coverage: %+v", model.Coverage)
			}
			for _, diagnostic := range diagnostics {
				checkRustProvenance(t, diagnostic.Provenance, request.Artifact)
			}
			foundUnsupported := false
			for _, diagnostic := range diagnostics {
				foundUnsupported = foundUnsupported || diagnostic.Code == semanticir.DiagnosticUnsupported || diagnostic.Code == semanticir.DiagnosticInvalidInput
			}
			if !foundUnsupported {
				t.Fatalf("construct was blocked without an unsupported/invalid-input diagnostic: %+v", diagnostics)
			}
		})
	}
}

func TestFrontendRustTestProjection(t *testing.T) {
	source := `fn classify(x: i32) -> bool { x == 0 }

#[test]
fn global_assertions() {
	assert!(classify(0));
}
`
	request := rustFrontendRequest(t, semanticir.ArtifactTests, source, nil)
	request.FiniteDomains = []semanticir.Domain{rustExactMembershipDomain(request.Artifact, "classify.x", "classify", "x", 0, 1)}
	zero := rustBoolOutcome(request.Artifact, "classify", false)
	one := rustBoolOutcome(request.Artifact, "classify", true)
	other := semanticir.OtherOutcome("classify", semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated))
	request.Outcomes = []semanticir.ObservableOutcome{zero, one, other}
	request.Operations = []semanticir.Operation{{ID: "classify", Kind: semanticir.OperationCallable, DomainIDs: []string{"classify.x"}, OutcomeIDs: []string{zero.ID, one.ID, other.ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "x", semanticir.TypeInteger, "classify.x")}}}
	rustFinalizeGroundings(&request)

	model, diagnostics := frontendrust.Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationComplete {
		t.Fatalf("Rust compiler-derived direct-call projection blocked: model=%+v diagnostics=%+v", model, diagnostics)
	}
	if model.TestProjection == nil || model.RunnerSelection == nil || !model.TestProjection.Complete || !model.RunnerSelection.Complete || len(model.Tests) != 1 || len(model.Tests[0].Assertions) != 1 {
		t.Fatalf("Rust direct-call test lacks complete compiler/runner evidence: %+v", model)
	}
	if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
		t.Fatalf("Rust compiler-derived test model is invalid: %+v", validation)
	}
	if validation := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(validation) {
		t.Fatalf("Rust compiler-derived test model leaves frozen scope: %+v\nrequest operations=%+v\nmodel operations=%+v", validation, request.Operations, model.Operations)
	}
}

func TestFrontendRustVerifierRelational(t *testing.T) {
	source := `fn choose(flag: bool) -> bool { flag }

#[test]
fn public_relation() {
    assert!(choose(false) != choose(true));
    assert!(choose(true));
}

#[test]
fn hidden_point() {
    assert!(!choose(false));
}
`
	request := rustFrontendRequest(t, semanticir.ArtifactTests, source, []semanticir.Domain{rustDomain("choose.flag", "false", "true")})
	returnedFalse := rustBoolOutcome(request.Artifact, "choose", false)
	returnedTrue := rustBoolOutcome(request.Artifact, "choose", true)
	other := semanticir.OtherOutcome("choose", semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated))
	request.Outcomes = []semanticir.ObservableOutcome{returnedFalse, returnedTrue, other}
	request.Operations = []semanticir.Operation{{ID: "choose", Kind: semanticir.OperationCallable, DomainIDs: []string{"choose.flag"}, OutcomeIDs: []string{returnedFalse.ID, returnedTrue.ID, other.ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "flag", semanticir.TypeBool, "choose.flag")}}}
	rustFinalizeGroundings(&request)

	model, diagnostics := frontendrust.Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationComplete {
		t.Fatalf("multi-test relational Rust verifier blocked: model=%+v diagnostics=%+v", model, diagnostics)
	}
	if len(model.Tests) != 2 || model.TestProjection == nil || len(model.TestProjection.PassRoots) != 2 || model.RunnerSelection == nil || len(model.RunnerSelection.TestIDs) != 2 {
		t.Fatalf("multi-test verifier projection/runner is incomplete: %+v", model)
	}
	if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
		t.Fatalf("multi-test relational Rust verifier is structurally invalid: %+v", validation)
	}
	if validation := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(validation) {
		t.Fatalf("multi-test relational Rust verifier leaves frozen scope: %+v", validation)
	}
}

func TestFrontendRustTestProjectionRejectsRepresentativeCategory(t *testing.T) {
	source := `fn special(x: i32) -> i32 { if x == 1 { 1 } else { 0 } }

#[test]
fn representative_is_not_category() {
    assert!(special(0) == 0);
}
`
	request := rustFrontendRequest(t, semanticir.ArtifactTests, source, nil)
	prov := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	zero := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}
	variable := semanticir.Expression{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: "x", Provenance: prov}
	literal := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &zero, Provenance: prov}
	membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpGE, Operands: []semanticir.Expression{variable, literal}, Provenance: prov}
	request.FiniteDomains = []semanticir.Domain{{
		ID: "special.x", Type: semanticir.TypeString, Provenance: prov,
		Values: []semanticir.DomainValue{{ID: "nonnegative", Groundings: []semanticir.GroundingAxiom{{OperationID: "special", Kind: semanticir.GroundingMembership, Membership: &membership, ConcreteWitness: map[string]semanticir.Literal{"x": zero}, Provenance: prov}}, Provenance: prov}},
	}}
	returnZero := rustReturnOutcome(request.Artifact, "special", 0)
	returnOne := rustReturnOutcome(request.Artifact, "special", 1)
	other := semanticir.OtherOutcome("special", prov)
	request.Outcomes = []semanticir.ObservableOutcome{returnZero, returnOne, other}
	request.Operations = []semanticir.Operation{{ID: "special", Kind: semanticir.OperationCallable, DomainIDs: []string{"special.x"}, OutcomeIDs: []string{returnZero.ID, returnOne.ID, other.ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "x", semanticir.TypeInteger, "special.x")}}}
	rustFinalizeGroundings(&request)

	model, diagnostics := frontendrust.Translate(context.Background(), request)
	if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked || model.TestProjection != nil {
		t.Fatalf("concrete x=0 test was unsoundly projected over non-singleton x>=0 category: model=%+v diagnostics=%+v", model, diagnostics)
	}
}

func TestFrontendRustUsesOnlyFrozenEnvironment(t *testing.T) {
	source := "fn choose(flag: bool) -> i32 { if flag { 1 } else { 0 } }\n"
	request := rustFrontendRequest(t, semanticir.ArtifactCode, source, []semanticir.Domain{rustDomain("choose.flag", "false", "true")})
	request.EntryPoints = []string{"choose"}
	request.Outcomes = []semanticir.ObservableOutcome{rustReturnOutcome(request.Artifact, "choose", 0), rustReturnOutcome(request.Artifact, "choose", 1), semanticir.OtherOutcome("choose", semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated))}
	request.Operations = []semanticir.Operation{{ID: "choose", Kind: semanticir.OperationCallable, DomainIDs: []string{"choose.flag"}, OutcomeIDs: []string{request.Outcomes[0].ID, request.Outcomes[1].ID, request.Outcomes[2].ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "flag", semanticir.TypeBool, "choose.flag")}}}
	rustFinalizeGroundings(&request)
	request.Workspace.Environment = []semanticir.EnvironmentVariable{{Name: "PATH", Value: ""}}
	request.Workspace.EnvironmentDigest, _ = semanticir.Digest(request.Workspace.Environment)
	model, diagnostics := frontendrust.Translate(context.Background(), request)
	if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
		t.Fatalf("ambient PATH leaked into Rust compiler execution: model=%+v diagnostics=%+v", model, diagnostics)
	}
	joined := fmt.Sprint(diagnostics)
	if !strings.Contains(joined, "linker") && !strings.Contains(joined, "compile") {
		t.Fatalf("frozen empty PATH blocked for an unrelated reason: %+v", diagnostics)
	}
}

func TestFrontendRustRealCrateFailsClosed(t *testing.T) {
	basePath := filepath.Join("..", "testdata", "e2e", "real-rust-jcode-picker-negative", "source", "solution", "src", "tui", "ui_inline_interactive.rs")
	baseBytes, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatalf("read real jcode ui_inline_interactive.rs: %v", err)
	}
	request := rustFrontendRequest(t, semanticir.ArtifactCode, string(baseBytes), nil)
	request.EntryPoints = []string{"truncate_display"}
	model, diagnostics := frontendrust.Translate(context.Background(), request)
	if !semanticir.HasErrors(diagnostics) || model.Coverage.Status != semanticir.TranslationBlocked {
		t.Fatalf("unsupported real-crate method was certified through a task-specific path: model=%+v diagnostics=%+v", model, diagnostics)
	}
}

func TestFrontendRustMaterialize(t *testing.T) {
	source := `fn choose(flag: bool) -> i32 {
    if flag { 1 } else { 0 }
}
`
	request := rustFrontendRequest(t, semanticir.ArtifactCode, source, []semanticir.Domain{rustDomain("choose.flag", "false", "true")})
	request.EntryPoints = []string{"choose"}
	request.Outcomes = []semanticir.ObservableOutcome{rustReturnOutcome(request.Artifact, "choose", 0), rustReturnOutcome(request.Artifact, "choose", 1), rustReturnOutcome(request.Artifact, "choose", 2), rustReturnOutcome(request.Artifact, "choose", 3), semanticir.OtherOutcome("choose", semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated))}
	request.Operations = []semanticir.Operation{{ID: "choose", Kind: semanticir.OperationCallable, DomainIDs: []string{"choose.flag"}, OutcomeIDs: []string{request.Outcomes[0].ID, request.Outcomes[1].ID, request.Outcomes[2].ID, request.Outcomes[3].ID, request.Outcomes[4].ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "flag", semanticir.TypeBool, "choose.flag")}}}
	rustFinalizeGroundings(&request)
	model, diagnostics := frontendrust.Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("Translate: %+v", diagnostics)
	}
	value := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 2}
	desired := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &value, OperationID: "choose", Provenance: model.Coverage.Provenance}
	desired.ID = semanticir.OutcomeID(desired)
	conditions := semanticir.Assignment{"choose.flag": "true"}
	trueInputs := map[string]semanticir.Literal{"flag": {Type: semanticir.TypeBool, Bool: true}}
	witness := semanticir.Counterexample{
		ID:         "choose-true-is-two",
		Obligation: semanticir.ObligationTestsSound,
		Choices: []semanticir.BehaviorChoice{{
			Behavior:  semanticir.BehaviorRef{OperationID: "choose", Conditions: conditions, Inputs: trueInputs, Provenance: model.Coverage.Provenance},
			OutcomeID: desired.ID,
		}},
		TestPasses: true,
		Provenance: model.Coverage.Provenance,
	}
	task := &semanticir.Task{ID: "rust-materialize", Operations: append([]semanticir.Operation(nil), model.Operations...), Outcomes: append([]semanticir.ObservableOutcome(nil), model.Outcomes...)}
	plan, materializeDiagnostics := frontendrust.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: task, Model: model, Counterexample: witness,
	})
	if semanticir.HasErrors(materializeDiagnostics) {
		t.Fatalf("Materialize blocked: %+v", materializeDiagnostics)
	}
	if len(plan.Edits) != 1 {
		t.Fatalf("edits = %d, want 1: %+v", len(plan.Edits), plan)
	}
	edit := plan.Edits[0]
	if len(edit.ExpectedBytes) == 0 || !strings.Contains(string(edit.Replacement), "== true { 2 }") {
		t.Fatalf("unexpected exact full-function Rust edit: %+v", edit)
	}
	if plan.Artifact != request.Artifact || plan.Provenance.ArtifactID != request.Artifact.ID || plan.Provenance.ArtifactDigest != request.Artifact.Digest || plan.Provenance.Translation != semanticir.TranslationTranslated {
		t.Fatalf("edit plan is not digest/provenance anchored: %+v", plan)
	}
	if plan.Expected.OperationID != "choose" || plan.Expected.OutcomeIDs[0] != desired.ID || !plan.Expected.TestPasses {
		t.Fatalf("expected semantics detached from witness: %+v", plan.Expected)
	}
	referenceConditions := semanticir.Assignment{"choose.flag": "false"}
	falseInputs := map[string]semanticir.Literal{"flag": {Type: semanticir.TypeBool, Bool: false}}
	referenceOutcome := request.Outcomes[0]
	referenceWitness := semanticir.Counterexample{
		ID: "choose-false-reference", Obligation: semanticir.ObligationReferenceCorrectness,
		OperationID: "choose", Conditions: referenceConditions,
		Choices:          []semanticir.BehaviorChoice{{Behavior: semanticir.BehaviorRef{OperationID: "choose", Conditions: referenceConditions, Inputs: falseInputs, Provenance: model.Coverage.Provenance}, OutcomeID: referenceOutcome.ID}},
		ObservedOutcomes: []string{referenceOutcome.ID}, ExpectedOutcomes: []string{referenceOutcome.ID}, Provenance: model.Coverage.Provenance,
	}
	probePlan, probeDiagnostics := frontendrust.GenerateProbe(context.Background(), semanticir.MaterializationRequest{Frontend: request, Task: task, Model: model, Counterexample: referenceWitness})
	if semanticir.HasErrors(probeDiagnostics) {
		t.Fatalf("GenerateProbe blocked: %+v", probeDiagnostics)
	}
	if probePlan.WitnessID != referenceWitness.ID || probePlan.Harness.SHA256 != semanticir.DigestBytes(probePlan.Harness.Bytes) || len(probePlan.Steps) != 2 || probePlan.Steps[0].Kind != executor.ProbeStepCompile || probePlan.Steps[1].Kind != executor.ProbeStepRun || len(probePlan.Operations) != 1 || len(probePlan.ExpectedSemantics.RuntimeOutcomes) != 1 {
		t.Fatalf("direct Rust probe is not fully witness/digest/step bound: %+v", probePlan)
	}
	for _, forbidden := range []string{referenceOutcome.ID, `"operation_id"`, `"provenance"`} {
		if bytes.Contains(probePlan.Harness.Bytes, []byte(forbidden)) {
			t.Fatalf("direct Rust probe harness self-reports semantic identity %q", forbidden)
		}
	}
	baseline := executor.TaskEnvironment{Command: []string{"/bin/sh", "-c", "true"}, WorkspaceRoot: request.Workspace.Root, WorkDir: request.Workspace.Root, Timeout: 10 * time.Second, Environment: []string{"PATH=" + os.Getenv("PATH")}, ExactEnvironment: true, PassSignal: executor.ExitCodeSignal(0)}
	report := executor.ConfirmProbes(context.Background(), baseline, []executor.ProbePlan{probePlan})
	if report.Status != executor.StatusConfirmed || len(report.Blockers) != 0 {
		t.Fatalf("direct Rust probe was not freshly confirmed: %+v", report)
	}
	modified := append([]byte(nil), request.Source...)
	modified = append(modified[:edit.StartByte], append(edit.Replacement, modified[edit.EndByte:]...)...)
	if semanticir.DigestBytes(modified) == request.Artifact.Digest {
		t.Fatal("materialized edit did not change the frozen artifact")
	}
	valueThree := semanticir.Literal{Type: semanticir.TypeInteger, Integer: 3}
	desiredThree := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &valueThree, OperationID: "choose", Provenance: model.Coverage.Provenance}
	desiredThree.ID = semanticir.OutcomeID(desiredThree)
	multiWitness := witness
	multiWitness.ID = "both-branches-change"
	multiWitness.Choices = []semanticir.BehaviorChoice{
		{Behavior: semanticir.BehaviorRef{OperationID: "choose", Conditions: semanticir.Assignment{"choose.flag": "true"}, Inputs: trueInputs, Provenance: model.Coverage.Provenance}, OutcomeID: desiredThree.ID},
		{Behavior: semanticir.BehaviorRef{OperationID: "choose", Conditions: semanticir.Assignment{"choose.flag": "false"}, Inputs: falseInputs, Provenance: model.Coverage.Provenance}, OutcomeID: desired.ID},
	}
	multiPlan, multiDiagnostics := frontendrust.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: task, Model: model, Counterexample: multiWitness,
	})
	if semanticir.HasErrors(multiDiagnostics) || len(multiPlan.Edits) != 1 || len(multiPlan.Expected.Choices) != 2 || multiPlan.Expected.OperationID != "" {
		t.Fatalf("multi-choice witness was not materialized atomically: plan=%+v diagnostics=%+v", multiPlan, multiDiagnostics)
	}

	bad := witness
	bad.Choices = append([]semanticir.BehaviorChoice(nil), witness.Choices...)
	bad.ID = "unknown-outcome"
	bad.Choices[0].OutcomeID = "outcome-does-not-exist"
	blockedPlan, blockedDiagnostics := frontendrust.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: task, Model: model, Counterexample: bad,
	})
	if len(blockedPlan.Edits) != 0 || !semanticir.HasErrors(blockedDiagnostics) {
		t.Fatalf("unrenderable witness did not block: plan=%+v diagnostics=%+v", blockedPlan, blockedDiagnostics)
	}
	wrongPoint := witness
	wrongPoint.ID = "conditions-inputs-disagree"
	wrongPoint.Choices = append([]semanticir.BehaviorChoice(nil), witness.Choices...)
	wrongPoint.Choices[0].Behavior.Inputs = falseInputs
	wrongPointPlan, wrongPointDiagnostics := frontendrust.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: task, Model: model, Counterexample: wrongPoint,
	})
	if len(wrongPointPlan.Edits) != 0 || !semanticir.HasErrors(wrongPointDiagnostics) {
		t.Fatalf("non-concrete behavior point did not block: plan=%+v diagnostics=%+v", wrongPointPlan, wrongPointDiagnostics)
	}
	foreignOutcome := rustReturnOutcome(request.Artifact, "foreign", 2)
	foreignOther := semanticir.OtherOutcome("foreign", model.Coverage.Provenance)
	foreignTask := *task
	foreignTask.Outcomes = append(append([]semanticir.ObservableOutcome(nil), task.Outcomes...), foreignOutcome, foreignOther)
	foreignTask.Operations = append(append([]semanticir.Operation(nil), task.Operations...), semanticir.Operation{ID: "foreign", Kind: semanticir.OperationCallable, OutcomeIDs: []string{foreignOutcome.ID, foreignOther.ID}})
	crossOperation := witness
	crossOperation.ID = "cross-operation-outcome"
	crossOperation.Choices = append([]semanticir.BehaviorChoice(nil), witness.Choices...)
	crossOperation.Choices[0].OutcomeID = foreignOutcome.ID
	crossPlan, crossDiagnostics := frontendrust.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: &foreignTask, Model: model, Counterexample: crossOperation,
	})
	if len(crossPlan.Edits) != 0 || !semanticir.HasErrors(crossDiagnostics) {
		t.Fatalf("cross-operation outcome did not block: plan=%+v diagnostics=%+v", crossPlan, crossDiagnostics)
	}
	other := witness
	other.Choices = append([]semanticir.BehaviorChoice(nil), witness.Choices...)
	other.ID = "canonical-other-without-concrete-trace"
	other.Choices[0].OutcomeID = request.Outcomes[4].ID
	otherPlan, otherDiagnostics := frontendrust.Materialize(context.Background(), semanticir.MaterializationRequest{
		Frontend: request, Task: task, Model: model, Counterexample: other,
	})
	if len(otherPlan.Edits) != 0 || !semanticir.HasErrors(otherDiagnostics) {
		t.Fatalf("canonical Other without a concrete terminal/effect witness did not block: plan=%+v diagnostics=%+v", otherPlan, otherDiagnostics)
	}
}

func TestFrontendRustMaterializeResultVariant(t *testing.T) {
	source := `fn decide(flag: bool) -> Result<i32, &'static str> {
    if flag { Ok(7) } else { Err("no") }
}
`
	request := rustFrontendRequest(t, semanticir.ArtifactCode, source, []semanticir.Domain{rustDomain("decide.flag", "false", "true")})
	request.EntryPoints = []string{"decide"}
	success := rustExceptionalOutcome(request.Artifact, "decide", semanticir.OutcomeSuccess, "", "")
	failure := rustExceptionalOutcome(request.Artifact, "decide", semanticir.OutcomeRaise, "Result::Err", "")
	other := semanticir.OtherOutcome("decide", semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated))
	request.Outcomes = []semanticir.ObservableOutcome{success, failure, other}
	request.Operations = []semanticir.Operation{{ID: "decide", Kind: semanticir.OperationCallable, DomainIDs: []string{"decide.flag"}, OutcomeIDs: []string{success.ID, failure.ID, other.ID}, Inputs: []semanticir.Variable{rustSpecInput(request.Artifact, "flag", semanticir.TypeBool, "decide.flag")}}}
	rustFinalizeGroundings(&request)
	model, diagnostics := frontendrust.Translate(context.Background(), request)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("Translate Result function: %+v", diagnostics)
	}
	conditions := semanticir.Assignment{"decide.flag": "false"}
	inputs := map[string]semanticir.Literal{"flag": {Type: semanticir.TypeBool, Bool: false}}
	witness := semanticir.Counterexample{
		ID: "decide-false-success", Obligation: semanticir.ObligationTestsSound, TestPasses: true, Provenance: model.Coverage.Provenance,
		Choices: []semanticir.BehaviorChoice{{Behavior: semanticir.BehaviorRef{OperationID: "decide", Conditions: conditions, Inputs: inputs, Provenance: model.Coverage.Provenance}, OutcomeID: success.ID}},
	}
	task := &semanticir.Task{ID: "rust-result-materialize", Operations: append([]semanticir.Operation(nil), model.Operations...), Outcomes: append([]semanticir.ObservableOutcome(nil), model.Outcomes...)}
	plan, materializeDiagnostics := frontendrust.Materialize(context.Background(), semanticir.MaterializationRequest{Frontend: request, Task: task, Model: model, Counterexample: witness})
	if semanticir.HasErrors(materializeDiagnostics) || len(plan.Edits) != 1 || !bytes.Contains(plan.Edits[0].Replacement, []byte("if flag == false { Ok(7) }")) {
		t.Fatalf("Result variant was not materialized by exact frozen terminal reuse: plan=%+v diagnostics=%+v", plan, materializeDiagnostics)
	}
}

func rustReturnOutcome(artifact semanticir.ArtifactRef, operationID string, value int64) semanticir.ObservableOutcome {
	literal := semanticir.Literal{Type: semanticir.TypeInteger, Integer: value}
	outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &literal, OperationID: operationID, Provenance: semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)}
	outcome.ID = semanticir.OutcomeID(outcome)
	return outcome
}

func rustBoolOutcome(artifact semanticir.ArtifactRef, operationID string, value bool) semanticir.ObservableOutcome {
	literal := semanticir.Literal{Type: semanticir.TypeBool, Bool: value}
	outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &literal, OperationID: operationID, Provenance: semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)}
	outcome.ID = semanticir.OutcomeID(outcome)
	return outcome
}

func rustStringOutcome(artifact semanticir.ArtifactRef, operationID, value string) semanticir.ObservableOutcome {
	literal := semanticir.Literal{Type: semanticir.TypeString, String: value}
	outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &literal, OperationID: operationID, Provenance: semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)}
	outcome.ID = semanticir.OutcomeID(outcome)
	return outcome
}

func rustExceptionalOutcome(artifact semanticir.ArtifactRef, operationID string, kind semanticir.OutcomeKind, exceptionType, message string) semanticir.ObservableOutcome {
	outcome := semanticir.ObservableOutcome{Kind: kind, ExceptionType: exceptionType, Message: message, OperationID: operationID, Provenance: semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)}
	outcome.ID = semanticir.OutcomeID(outcome)
	return outcome
}

func rustSpecInput(artifact semanticir.ArtifactRef, name string, valueType semanticir.ValueType, domainID string) semanticir.Variable {
	return semanticir.Variable{Name: name, Type: valueType, DomainID: domainID, Provenance: semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)}
}

func rustFrontendRequest(t *testing.T, kind semanticir.ArtifactKind, source string, domains []semanticir.Domain) semanticir.FrontendRequest {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	path := "artifact.rs"
	if err := os.WriteFile(filepath.Join(root, path), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := semanticir.ArtifactRef{ID: "rust-" + string(kind), Kind: kind, Path: path, Digest: semanticir.DigestBytes([]byte(source))}
	translator := pinnedRustc(t)
	prover := pinnedZ3(t)
	workspaceProvenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	entry := semanticir.WorkspaceEntry{Path: path, Artifact: artifact, Provenance: workspaceProvenance}
	var runner semanticir.ToolRef
	var runnerCommand *semanticir.WorkspaceCommand
	var configuration *semanticir.ArtifactRef
	if kind == semanticir.ArtifactTests {
		manifestPath := "Cargo.toml"
		manifest := []byte("[package]\nname = \"ray_frontend_tests\"\nversion = \"0.0.0\"\nedition = \"2021\"\n\n[lib]\npath = \"artifact.rs\"\n")
		if err := os.WriteFile(filepath.Join(root, manifestPath), manifest, 0o600); err != nil {
			t.Fatal(err)
		}
		config := semanticir.ArtifactRef{ID: "rust-test-manifest", Kind: semanticir.ArtifactConfiguration, Path: manifestPath, Digest: semanticir.DigestBytes(manifest)}
		configProvenance := semanticir.NewProvenance(config, semanticir.SourceLocation{Path: manifestPath, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
		entry = semanticir.WorkspaceEntry{Path: path, Artifact: artifact, Provenance: workspaceProvenance}
		runner = pinnedCargo(t)
		configuration = &config
		_ = configProvenance
	}
	treeDigest, err := executor.WorkspaceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	environment := []semanticir.EnvironmentVariable{{Name: "PATH", Value: os.Getenv("PATH")}}
	environmentDigest, err := semanticir.Digest(environment)
	if err != nil {
		t.Fatal(err)
	}
	if kind == semanticir.ArtifactTests {
		config := *configuration
		configProvenance := semanticir.NewProvenance(config, semanticir.SourceLocation{Path: config.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
		runnerCommand = &semanticir.WorkspaceCommand{ID: "rust-libtest", WorkspaceID: "rust-workspace", State: semanticir.WorkspaceSolutionNewTests, TreeDigest: treeDigest, Command: strings.Join([]string{runner.Path, "test", "--manifest-path", config.Path, "--lib", "--", "--test-threads=1"}, " "), WorkingDirectory: ".", Environment: append([]semanticir.EnvironmentVariable(nil), environment...), EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 30000, PassSignal: semanticir.PassSignal{Kind: semanticir.PassSignalExitCode, Expected: "0", Provenance: configProvenance}, ExpectedPass: true, Tools: []semanticir.ToolRef{runner}, Provenance: configProvenance}
	}
	entries := []semanticir.WorkspaceEntry{entry}
	if configuration != nil {
		configProvenance := semanticir.NewProvenance(*configuration, semanticir.SourceLocation{Path: configuration.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
		entries = append(entries, semanticir.WorkspaceEntry{Path: configuration.Path, Artifact: *configuration, Provenance: configProvenance})
	}
	return semanticir.FrontendRequest{
		TaskID: "frontend-rust", Artifact: artifact, Language: semanticir.LanguageRust, Kind: kind,
		Source: []byte(source), FiniteDomains: domains, Translator: translator, Prover: prover, Runner: runner, RunnerCommand: runnerCommand, Configuration: configuration,
		Workspace: semanticir.WorkspaceRef{
			ID: "rust-workspace", State: semanticir.WorkspaceSolutionNewTests, Root: root,
			TreeDigest: treeDigest, WorkingDirectory: ".", BuildCommand: "rustc",
			Environment: environment, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true,
			Entries: entries, Provenance: workspaceProvenance,
		},
		FocusArtifacts: []semanticir.ArtifactRef{artifact},
		ChangedRanges:  []semanticir.ChangedSourceRange{{ArtifactID: artifact.ID, Path: artifact.Path, StartLine: 1, EndLine: strings.Count(source, "\n") + 1, SliceDigest: artifact.Digest, Provenance: workspaceProvenance}},
	}
}

func pinnedCargo(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is required for strict Rust test-evidence tests")
	}
	if selected, selectedErr := exec.Command("rustup", "which", "cargo").Output(); selectedErr == nil {
		path = strings.TrimSpace(string(selected))
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(path, "--version", "--verbose")
	command.Env = []string{"PATH=" + os.Getenv("PATH")}
	version, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("cargo --version --verbose: %v: %s", err, version)
	}
	return semanticir.ToolRef{Name: "cargo", Path: path, Digest: semanticir.DigestBytes(body), Version: strings.TrimSpace(string(version))}
}

func pinnedZ3(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath("z3")
	if err != nil {
		t.Skip("z3 is required for replayable Rust frontend tests")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(path, "--version")
	command.Env = []string{"PATH=" + os.Getenv("PATH")}
	version, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("z3 --version: %v: %s", err, version)
	}
	return semanticir.ToolRef{Name: "z3", Path: path, Digest: semanticir.DigestBytes(body), Version: strings.TrimSpace(string(version))}
}

func pinnedRustc(t *testing.T) semanticir.ToolRef {
	t.Helper()
	path, err := exec.LookPath("rustc")
	if err != nil {
		t.Skip("rustc is required for strict Rust frontend tests")
	}
	if selected, selectedErr := exec.Command("rustup", "which", "rustc").Output(); selectedErr == nil {
		path = strings.TrimSpace(string(selected))
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(path, "--version", "--verbose")
	command.Env = []string{}
	version, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("rustc --version --verbose: %v: %s", err, version)
	}
	return semanticir.ToolRef{Name: "rustc", Path: path, Digest: semanticir.DigestBytes(binary), Version: strings.TrimSpace(string(version))}
}

func rustDomain(id string, values ...string) semanticir.Domain {
	domain := semanticir.Domain{ID: id, Type: semanticir.TypeString}
	allBool, allInteger := true, true
	for _, value := range values {
		allBool = allBool && (value == "true" || value == "false")
		integer := false
		if _, err := strconv.ParseInt(value, 0, 64); err == nil {
			integer = true
		}
		allInteger = allInteger && integer
	}
	if allBool {
		domain.Type = semanticir.TypeBool
	} else if allInteger {
		domain.Type = semanticir.TypeInteger
	}
	for _, value := range values {
		literal, ok := rustTestLiteral(domain.Type, value)
		if ok {
			domain.Values = append(domain.Values, semanticir.DomainValue{ID: value, Value: &literal})
		} else {
			domain.Values = append(domain.Values, semanticir.DomainValue{ID: value})
		}
	}
	return domain
}

func rustFinalizeGroundings(request *semanticir.FrontendRequest) {
	prov := semanticir.NewProvenance(request.Artifact, semanticir.SourceLocation{Path: request.Artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	for domainIndex := range request.FiniteDomains {
		domain := &request.FiniteDomains[domainIndex]
		for valueIndex := range domain.Values {
			value := &domain.Values[valueIndex]
			value.Provenance = prov
			for groundingIndex := range value.Groundings {
				grounding := &value.Groundings[groundingIndex]
				grounding.Provenance = prov
				if grounding.Membership != nil {
					setRustExpressionProvenance(grounding.Membership, prov)
				}
			}
			if value.Value == nil || len(value.Groundings) != 0 {
				continue
			}
			operationID, inputName, found := strings.Cut(domain.ID, ".")
			if !found || operationID == "" || inputName == "" {
				continue
			}
			literalValue := *value.Value
			variable := semanticir.Expression{Kind: semanticir.ExprVariable, Type: literalValue.Type, Name: inputName, Provenance: prov}
			literal := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: literalValue.Type, Literal: &literalValue, Provenance: prov}
			membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{variable, literal}, Provenance: prov}
			value.Value = nil
			value.Groundings = []semanticir.GroundingAxiom{{OperationID: operationID, Kind: semanticir.GroundingMembership, Membership: &membership, ConcreteWitness: map[string]semanticir.Literal{inputName: literalValue}, Provenance: prov}}
		}
		domain.Type = semanticir.TypeString
		domain.Provenance = prov
	}
	request.Groundings = nil
	for _, operation := range request.Operations {
		domains := make([]semanticir.Domain, 0, len(operation.DomainIDs))
		for _, domainID := range operation.DomainIDs {
			domain, exists := rustTestDomain(request.FiniteDomains, domainID)
			if !exists {
				continue
			}
			domains = append(domains, domain)
		}
		indices := make([]int, len(domains))
		combinations := 1
		for _, domain := range domains {
			combinations *= len(domain.Values)
		}
		for combination := 0; combination < combinations; combination++ {
			conditions := semanticir.Assignment{}
			inputs := map[string]semanticir.Literal{}
			consistent := true
			for index, domain := range domains {
				value := domain.Values[indices[index]]
				conditions[domain.ID] = value.ID
				for _, axiom := range value.Groundings {
					if axiom.OperationID != operation.ID {
						continue
					}
					for name, literal := range axiom.ConcreteWitness {
						if previous, exists := inputs[name]; exists && previous != literal {
							consistent = false
						}
						inputs[name] = literal
					}
				}
			}
			if consistent && len(inputs) == len(operation.Inputs) && !rustTestExcluded(request.Constraints, operation.ID, conditions) {
				request.Groundings = append(request.Groundings, semanticir.AssignmentGrounding{ID: semanticir.AssignmentGroundingID(operation.ID, conditions), OperationID: operation.ID, Conditions: conditions, Inputs: inputs, Provenance: prov})
			}
			for index := len(indices) - 1; index >= 0; index-- {
				indices[index]++
				if indices[index] < len(domains[index].Values) {
					break
				}
				indices[index] = 0
			}
		}
	}
}

func setRustExpressionProvenance(expression *semanticir.Expression, prov semanticir.Provenance) {
	expression.Provenance = prov
	for index := range expression.Operands {
		setRustExpressionProvenance(&expression.Operands[index], prov)
	}
}

func rustTestDomain(domains []semanticir.Domain, id string) (semanticir.Domain, bool) {
	for _, domain := range domains {
		if domain.ID == id {
			return domain, true
		}
	}
	return semanticir.Domain{}, false
}

func rustTestExcluded(constraints []semanticir.Constraint, operationID string, conditions semanticir.Assignment) bool {
	for _, constraint := range constraints {
		if constraint.OperationID != operationID || len(constraint.Conditions) != len(conditions) {
			continue
		}
		equal := true
		for domainID, valueID := range conditions {
			equal = equal && constraint.Conditions[domainID] == valueID
		}
		if equal {
			return true
		}
	}
	return false
}

func rustExactMembershipDomain(artifact semanticir.ArtifactRef, domainID, operationID, inputName string, values ...int64) semanticir.Domain {
	prov := semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
	domain := semanticir.Domain{ID: domainID, Type: semanticir.TypeString, Provenance: prov}
	for _, integer := range values {
		value := semanticir.Literal{Type: semanticir.TypeInteger, Integer: integer}
		variable := semanticir.Expression{Kind: semanticir.ExprVariable, Type: semanticir.TypeInteger, Name: inputName, Provenance: prov}
		literal := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &value, Provenance: prov}
		membership := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{variable, literal}, Provenance: prov}
		domain.Values = append(domain.Values, semanticir.DomainValue{ID: strconv.FormatInt(integer, 10), Groundings: []semanticir.GroundingAxiom{{OperationID: operationID, Kind: semanticir.GroundingMembership, Membership: &membership, ConcreteWitness: map[string]semanticir.Literal{inputName: value}, Provenance: prov}}, Provenance: prov})
	}
	return domain
}

func rustTestLiteral(valueType semanticir.ValueType, value string) (semanticir.Literal, bool) {
	switch valueType {
	case semanticir.TypeBool:
		parsed, err := strconv.ParseBool(value)
		return semanticir.Literal{Type: valueType, Bool: parsed}, err == nil
	case semanticir.TypeInteger:
		parsed, err := strconv.ParseInt(value, 0, 64)
		return semanticir.Literal{Type: valueType, Integer: parsed}, err == nil
	case semanticir.TypeString:
		return semanticir.Literal{Type: valueType, String: value}, true
	default:
		return semanticir.Literal{}, false
	}
}

func rustOperation(t *testing.T, model semanticir.ArtifactModel, id string) semanticir.Operation {
	t.Helper()
	for _, operation := range model.Operations {
		if operation.ID == id {
			return operation
		}
	}
	t.Fatalf("operation %q absent: %+v", id, model.Operations)
	return semanticir.Operation{}
}

func walkRustStatements(t *testing.T, statements []semanticir.Statement, artifact semanticir.ArtifactRef, statementKinds map[semanticir.StatementKind]bool, expressionKinds map[semanticir.ExpressionKind]bool) {
	t.Helper()
	for _, statement := range statements {
		statementKinds[statement.Kind] = true
		checkRustProvenance(t, statement.Provenance, artifact)
		if statement.Condition != nil {
			walkRustExpression(t, *statement.Condition, artifact, expressionKinds)
		}
		if statement.Value != nil {
			walkRustExpression(t, *statement.Value, artifact, expressionKinds)
		}
		walkRustStatements(t, statement.Then, artifact, statementKinds, expressionKinds)
		walkRustStatements(t, statement.Else, artifact, statementKinds, expressionKinds)
	}
}

func walkRustExpression(t *testing.T, expression semanticir.Expression, artifact semanticir.ArtifactRef, kinds map[semanticir.ExpressionKind]bool) {
	t.Helper()
	kinds[expression.Kind] = true
	checkRustProvenance(t, expression.Provenance, artifact)
	for _, operand := range expression.Operands {
		walkRustExpression(t, operand, artifact, kinds)
	}
}

func checkRustProvenance(t *testing.T, provenance semanticir.Provenance, artifact semanticir.ArtifactRef) {
	t.Helper()
	if provenance.ArtifactID != artifact.ID || provenance.ArtifactDigest != artifact.Digest || provenance.Location.Path != artifact.Path || provenance.Location.StartLine < 1 || provenance.Location.StartColumn < 1 || provenance.Translation == semanticir.TranslationUnknown {
		t.Errorf("invalid Rust provenance: %+v (artifact %+v)", provenance, artifact)
	}
}
