package tests

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/testir"
)

func TestMain(m *testing.M) {
	for _, argument := range os.Args[1:] {
		const rawPrefix = "-proof-replay-raw="
		if strings.HasPrefix(argument, rawPrefix) {
			body, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(argument, rawPrefix))
			if err != nil {
				os.Exit(2)
			}
			_, _ = io.Copy(io.Discard, os.Stdin)
			_, _ = os.Stdout.Write(body)
			os.Exit(0)
		}
		const prefix = "-proof-replay-helper="
		if strings.HasPrefix(argument, prefix) {
			_, _ = io.Copy(io.Discard, os.Stdin)
			_, _ = os.Stdout.Write([]byte(strings.TrimPrefix(argument, prefix) + "\n"))
			os.Exit(0)
		}
	}
	code := m.Run()
	if proofWorkspacePath != "" {
		_ = os.RemoveAll(proofWorkspacePath)
	}
	os.Exit(code)
}

func TestProofReference(t *testing.T) {
	task := proofTask([]string{"good"}, andPredicate(
		outcomeIn("x0", "f", assignment("x", "0"), "good"),
		outcomeIn("x1", "f", assignment("x", "1"), "good"),
	))
	assertProofVerdict(t, proof.Verify(context.Background(), task), proof.VerdictVerified)
	if result := proof.Verify(context.Background(), task); result.ReferenceAcceptance.Verdict != proof.VerdictVerified {
		t.Fatalf("reference acceptance = %+v, want VERIFIED independently", result.ReferenceAcceptance)
	}

	task.CodeCases[1].OutcomeIDs = []string{"bad"}
	syncProofArtifacts(task)
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.Reference.Verdict != proof.VerdictNotVerified || result.Reference.Witness == nil {
		t.Fatalf("reference result = %+v, want concrete refutation", result.Reference)
	}
	witness := result.Reference.Witness
	if witness.OperationID != "f" || witness.Conditions["x"] != "1" || len(witness.Choices) != 2 || witness.ObservedOutcomes[1] != proofOutcomeAlias(task.Outcomes, "bad") {
		t.Fatalf("reference witness = %+v, want full concrete behavior vector", witness)
	}
	if result.FalsePositive.Verdict != proof.VerdictVerified || result.Fairness.Verdict != proof.VerdictVerified {
		t.Fatalf("independent obligations changed: soundness=%s fairness=%s", result.FalsePositive.Verdict, result.Fairness.Verdict)
	}
}

func TestProofReferenceAcceptance(t *testing.T) {
	task := proofTask([]string{"good"}, falsePredicate())
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.Reference.Verdict != proof.VerdictVerified {
		t.Fatalf("reference correctness = %+v, want VERIFIED", result.Reference)
	}
	if result.ReferenceAcceptance.Verdict != proof.VerdictNotVerified || result.ReferenceAcceptance.Witness == nil || result.ReferenceAcceptance.Witness.TestPasses {
		t.Fatalf("reference acceptance = %+v, want rejected exact C witness", result.ReferenceAcceptance)
	}
	if len(result.ReferenceAcceptance.Witness.Choices) != len(task.CodeCases) {
		t.Fatalf("reference acceptance witness has %d choices, want complete C vector of %d", len(result.ReferenceAcceptance.Witness.Choices), len(task.CodeCases))
	}
}

func TestProofFalsePositive(t *testing.T) {
	task := proofTask([]string{"good"}, truePredicate())
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.FalsePositive.Verdict != proof.VerdictNotVerified || result.FalsePositive.Witness == nil {
		t.Fatalf("soundness result = %+v, want false-positive witness", result.FalsePositive)
	}
	witness := result.FalsePositive.Witness
	if !witness.TestPasses || len(witness.Choices) != 2 || witness.Choices[0].OutcomeID == "good" {
		t.Fatalf("false-positive witness = %+v, want minimal passing forbidden vector", witness)
	}
	if result.FalsePositive.OutcomeChecks != 32 {
		t.Fatalf("soundness checked %d outcome components, want exhaustive 16 vectors * 2 cases", result.FalsePositive.OutcomeChecks)
	}

	emptySuite := proofTask([]string{"good"}, truePredicate())
	emptySuite.Tests = []semanticir.TestModel{{ID: "explicit-empty-suite", Predicate: truePredicate(), Provenance: proofProvenance(emptySuite.Artifacts[1].Artifact)}}
	syncProofArtifacts(emptySuite)
	emptyResult := proof.Verify(context.Background(), emptySuite)
	if emptyResult.FalsePositive.Verdict != proof.VerdictNotVerified {
		t.Fatalf("empty test conjunction must accept every vector, got %+v", emptyResult.FalsePositive)
	}
}

func TestProofFairness(t *testing.T) {
	testProofRelational(t)
}

func TestProofFalseNegative(t *testing.T) {
	testProofRelational(t)
}

func TestProofRelational(t *testing.T) {
	testProofRelational(t)
}

func testProofRelational(t *testing.T) {
	task := proofTask([]string{"alt", "good"}, andPredicate(
		outcomeIn("domain0", "f", assignment("x", "0"), "alt", "good"),
		outcomeIn("domain1", "f", assignment("x", "1"), "alt", "good"),
		outcomeEqual("same", "f", assignment("x", "0"), "f", assignment("x", "1")),
	))
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.Reference.Verdict != proof.VerdictVerified || result.FalsePositive.Verdict != proof.VerdictVerified {
		t.Fatalf("relational test should be sound for the spec: reference=%s soundness=%s", result.Reference.Verdict, result.FalsePositive.Verdict)
	}
	if result.Fairness.Verdict != proof.VerdictNotVerified || result.Fairness.Witness == nil {
		t.Fatalf("fairness result = %+v, want relational witness", result.Fairness)
	}
	witness := result.Fairness.Witness
	if witness.TestPasses || len(witness.Choices) != 2 || witness.Choices[0].OutcomeID == witness.Choices[1].OutcomeID {
		t.Fatalf("fairness witness = %+v, want complete unequal spec-compliant vector", witness)
	}
}

func TestProofMultiOperation(t *testing.T) {
	task := proofTask([]string{"good"}, andPredicate(
		outcomeIn("f0", "f", assignment("x", "0"), "good"),
		outcomeIn("f1", "f", assignment("x", "1"), "good"),
	))
	spec := task.Spec
	code := task.Artifacts[0].Artifact
	tests := task.Artifacts[1].Artifact
	task.Domains = append(task.Domains, semanticir.Domain{ID: "mode", Type: semanticir.TypeInteger, Provenance: proofProvenance(spec), Values: []semanticir.DomainValue{
		{ID: "p", Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}, Provenance: proofProvenance(spec)}, {ID: "q", Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}, Provenance: proofProvenance(spec)}, {ID: "r", Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 2}, Provenance: proofProvenance(spec)},
	}})
	task.Outcomes = append(task.Outcomes,
		semanticir.ObservableOutcome{ID: "C", Kind: semanticir.OutcomeReturn, OperationID: "g", Value: &semanticir.Literal{Type: semanticir.TypeString, String: "C"}, Provenance: proofProvenance(spec)},
		semanticir.ObservableOutcome{ID: "D", Kind: semanticir.OutcomeReturn, OperationID: "g", Value: &semanticir.Literal{Type: semanticir.TypeString, String: "D"}, Provenance: proofProvenance(spec)},
		semanticir.OtherOutcome("g", proofProvenance(spec)),
	)
	gOther := semanticir.OtherOutcome("g", proofProvenance(spec)).ID
	task.Operations = append(task.Operations, semanticir.Operation{
		ID: "g", Kind: semanticir.OperationFunction, DomainIDs: []string{"mode"}, OutcomeIDs: []string{"C", "D", gOther},
		Inputs: []semanticir.Variable{{Name: "mode", Type: semanticir.TypeInteger, DomainID: "mode", Provenance: proofProvenance(spec)}}, Provenance: proofProvenance(spec),
	})
	children := append([]semanticir.TestPredicate(nil), task.Tests[0].Predicate.Children...)
	for _, mode := range []string{"p", "q", "r"} {
		task.Requirements = append(task.Requirements, semanticir.RequirementCase{
			ID: "req-g-" + mode, OperationID: "g", Conditions: assignment("mode", mode), RequiredOutcomes: []string{"C"}, ForbiddenOutcomes: []string{"D", gOther},
			InstructionClauseIDs: []string{"clause-0"}, InstructionSources: []semanticir.Provenance{proofProvenance(task.Instruction)}, Evidence: []semanticir.Provenance{proofProvenance(code)}, Provenance: proofProvenance(spec),
		})
		task.CodeCases = append(task.CodeCases, semanticir.BehaviorCase{ID: "code-g-" + mode, OperationID: "g", Conditions: assignment("mode", mode), Inputs: proofBehaviorInputs(assignment("mode", mode)), OutcomeIDs: []string{"C"}, Provenance: proofProvenance(code)})
		children = append(children, outcomeIn("g-"+mode, "g", assignment("mode", mode), "C"))
	}
	task.Tests[0].Predicate = semanticir.TestPredicate{Kind: semanticir.PredicateAnd, Children: children, Provenance: proofProvenance(tests)}
	syncProofArtifacts(task)
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictVerified)
	if result.Transcript.ReachableCases != 5 {
		t.Fatalf("reachable cases = %d, want f:{0,1} plus g:{p,q,r}", result.Transcript.ReachableCases)
	}
}

func TestProofZeroDomainOperation(t *testing.T) {
	task := proofTask([]string{"good"}, outcomeIn("initial", "f", assignment("x", "0"), "good"))
	task.Operations[0].DomainIDs = nil
	task.Operations[0].Inputs = nil
	task.Requirements = task.Requirements[:1]
	task.Requirements[0].ID = "req-unit"
	task.Requirements[0].Conditions = semanticir.Assignment{}
	task.CodeCases = task.CodeCases[:1]
	task.CodeCases[0].ID = "code-unit"
	task.CodeCases[0].Conditions = semanticir.Assignment{}
	task.CodeCases[0].Inputs = nil
	task.Tests[0].Predicate = outcomeIn("unit", "f", semanticir.Assignment{}, "good")
	syncProofArtifacts(task)
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictVerified)
	if result.Transcript.ReachableCases != 1 || len(result.Transcript.CompilerEvidence[0].OperationScopes) != 1 || len(result.Transcript.CompilerEvidence[0].Partitions) != 0 {
		t.Fatalf("zero-domain transcript = %+v, want one scoped case and no domain partitions", result.Transcript)
	}
}

func TestProofEffects(t *testing.T) {
	task := proofTask([]string{"alt", "good"}, andPredicate(
		hasEffect("e0", "f", assignment("x", "0"), semanticir.EffectWrite, "audit"),
		hasEffect("e1", "f", assignment("x", "1"), semanticir.EffectWrite, "audit"),
	))
	effect := semanticir.Effect{ID: "audit-write", Kind: semanticir.EffectWrite, Target: "audit", Provenance: proofProvenance(task.Spec)}
	task.Outcomes[0].Effects = []semanticir.Effect{effect}
	for i := range task.Requirements {
		task.Requirements[i].Effects = []semanticir.Effect{effect}
	}
	syncProofArtifacts(task)
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.Reference.Verdict != proof.VerdictNotVerified || result.FalsePositive.Verdict != proof.VerdictVerified || result.Fairness.Verdict != proof.VerdictVerified {
		t.Fatalf("effect obligations: reference=%s soundness=%s fairness=%s", result.Reference.Verdict, result.FalsePositive.Verdict, result.Fairness.Verdict)
	}
}

func TestProofOutcomeClosure(t *testing.T) {
	otherID := semanticir.OtherOutcome("f", proofProvenance(proofArtifact("spec", semanticir.ArtifactSpec))).ID
	task := proofTask([]string{"good"}, andPredicate(
		outcomeIn("other-0", "f", assignment("x", "0"), otherID),
		outcomeIn("other-1", "f", assignment("x", "1"), otherID),
	))
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.FalsePositive.Witness == nil || !proofWitnessContainsOutcome(result.FalsePositive.Witness, otherID) {
		t.Fatalf("undeclared concrete behavior complement = %+v, want explicit false-positive other-outcome witness", result.FalsePositive.Witness)
	}

	missing := proofTask([]string{"good"}, truePredicate())
	graph := missing.Artifacts[0].CompilerEvidence[0].SemanticGraph
	graph.Nodes[len(graph.Nodes)-1].Kind = semanticir.CompilerNodeSuccess
	assertProofEngineBlocked(t, proof.Verify(context.Background(), missing), "invalid-reference-ir")
}

func TestProofEffectValues(t *testing.T) {
	task := proofTask([]string{"alt", "good"}, andPredicate(
		hasEffectValue("v0", "f", assignment("x", "0"), "expected"),
		hasEffectValue("v1", "f", assignment("x", "1"), "expected"),
	))
	expected := literalStringExpression(task.Spec, "expected")
	wrong := literalStringExpression(task.Spec, "wrong")
	task.Outcomes[0].Effects = []semanticir.Effect{{ID: "audit-write", Kind: semanticir.EffectWrite, Target: "audit", Value: &expected, Provenance: proofProvenance(task.Spec)}}
	task.Outcomes[2].Effects = []semanticir.Effect{{ID: "audit-write", Kind: semanticir.EffectWrite, Target: "audit", Value: &wrong, Provenance: proofProvenance(task.Spec)}}
	for i := range task.Requirements {
		task.Requirements[i].Effects = []semanticir.Effect{{ID: "requirement-audit-write", Kind: semanticir.EffectWrite, Target: "audit", Value: &expected, Provenance: proofProvenance(task.Spec)}}
		task.CodeCases[i].OutcomeIDs = []string{"alt"}
	}
	syncProofArtifacts(task)
	assertProofVerdict(t, proof.Verify(context.Background(), task), proof.VerdictVerified)

	task.Tests[0].Predicate = andPredicate(
		hasEffectValue("v0-wrong", "f", assignment("x", "0"), "wrong"),
		hasEffectValue("v1-wrong", "f", assignment("x", "1"), "wrong"),
	)
	syncProofArtifacts(task)
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.FalsePositive.Verdict != proof.VerdictNotVerified || result.Fairness.Verdict != proof.VerdictNotVerified {
		t.Fatalf("wrong effect value must refute soundness and fairness: soundness=%s fairness=%s", result.FalsePositive.Verdict, result.Fairness.Verdict)
	}
}

func TestProofInvariants(t *testing.T) {
	task := proofTask([]string{"alt", "good"}, andPredicate(
		outcomeIn("i0", "f", assignment("x", "0"), "good"),
		outcomeIn("i1", "f", assignment("x", "1"), "good"),
	))
	variable := semanticir.Expression{Kind: semanticir.ExprVariable, Type: semanticir.TypeString, Name: "result", Provenance: proofProvenance(task.Spec)}
	want := literalStringExpression(task.Spec, "good")
	task.Invariants = []semanticir.Invariant{{
		ID: "result-good", Predicate: semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{variable, want}, Provenance: proofProvenance(task.Spec)},
		Bindings: []semanticir.InvariantBinding{{Variable: "result", Kind: semanticir.BindOutcomeValue, Provenance: proofProvenance(task.Spec)}}, Provenance: proofProvenance(task.Spec),
	}}
	for i := range task.Requirements {
		task.Requirements[i].InvariantIDs = []string{"result-good"}
	}
	syncProofArtifacts(task)
	assertProofVerdict(t, proof.Verify(context.Background(), task), proof.VerdictVerified)
}

func TestProofExpressionSemantics(t *testing.T) {
	task := proofTask([]string{"good"}, andPredicate(
		outcomeIn("e0", "f", assignment("x", "0"), "good"),
		outcomeIn("e1", "f", assignment("x", "1"), "good"),
	))
	provenance := proofProvenance(task.Spec)
	truth := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeBool, Literal: &semanticir.Literal{Type: semanticir.TypeBool, Bool: true}, Provenance: provenance}
	deadCall := semanticir.Expression{Kind: semanticir.ExprCall, Type: semanticir.TypeBool, Name: "must-not-run", Provenance: provenance}
	shortCircuit := semanticir.Expression{Kind: semanticir.ExprBool, Type: semanticir.TypeBool, Operator: semanticir.OpOr, Operands: []semanticir.Expression{truth, deadCall, deadCall}, Provenance: provenance}
	nullOptional := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeOptional, Literal: &semanticir.Literal{Type: semanticir.TypeOptional, Null: true}, Provenance: provenance}
	isNull := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpIsNull, Operands: []semanticir.Expression{nullOptional}, Provenance: provenance}
	task.Invariants = []semanticir.Invariant{{
		ID: "short-circuit-and-null", Predicate: semanticir.Expression{Kind: semanticir.ExprBool, Type: semanticir.TypeBool, Operator: semanticir.OpAnd, Operands: []semanticir.Expression{isNull, shortCircuit, truth}, Provenance: provenance}, Provenance: provenance,
	}}
	for index := range task.Requirements {
		task.Requirements[index].InvariantIDs = []string{"short-circuit-and-null"}
	}
	syncProofArtifacts(task)
	assertProofVerdict(t, proof.Verify(context.Background(), task), proof.VerdictVerified)
}

func TestProofRequirementConjunction(t *testing.T) {
	task := proofTask([]string{"alt", "good"}, andPredicate(
		outcomeIn("f0", "f", assignment("x", "0"), "good"),
		outcomeIn("f1", "f", assignment("x", "1"), "good"),
	))
	for _, value := range []string{"0", "1"} {
		task.Requirements = append(task.Requirements, semanticir.RequirementCase{
			ID: "narrow-" + value, OperationID: "f", Conditions: assignment("x", value), RequiredOutcomes: []string{"good"}, ForbiddenOutcomes: []string{"alt", "bad", semanticir.OtherOutcome("f", proofProvenance(task.Spec)).ID},
			InstructionClauseIDs: []string{"clause-" + value}, InstructionSources: []semanticir.Provenance{proofProvenance(task.Instruction)}, Evidence: []semanticir.Provenance{proofProvenance(task.Artifacts[0].Artifact)}, Provenance: proofProvenance(task.Spec),
		})
	}
	syncProofArtifacts(task)
	assertProofVerdict(t, proof.Verify(context.Background(), task), proof.VerdictVerified)
}

func TestProofEmptyForbiddenPartition(t *testing.T) {
	task := proofTask([]string{"alt", "bad", "good", semanticir.OtherOutcome("f", proofProvenance(proofArtifact("spec", semanticir.ArtifactSpec))).ID}, truePredicate())
	for index := range task.Requirements {
		task.Requirements[index].ForbiddenOutcomes = nil
	}
	syncProofArtifacts(task)
	assertProofVerdict(t, proof.Verify(context.Background(), task), proof.VerdictVerified)
}

func TestProofFlattened(t *testing.T) {
	task := proofTask([]string{"good"}, truePredicate())
	task.CodeCases[0].OutcomeIDs = []string{"bad"}
	assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "conflicting-code-model")
}

func TestProofMissingSpecInputsBlocked(t *testing.T) {
	task := proofTask([]string{"good"}, andPredicate(
		outcomeIn("x0", "f", assignment("x", "0"), "good"),
		outcomeIn("x1", "f", assignment("x", "1"), "good"),
	))
	// The complete finite input vocabulary belongs to frozen Spec IR; a
	// frontend may not silently invent it after the semantic freeze.
	task.Operations[0].Inputs = nil
	refreshProofSpecIR(task)
	assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "incomplete-domain-grounding")
}

func TestProofPredicateFalse(t *testing.T) {
	task := proofTask([]string{"good"}, semanticir.TestPredicate{
		Kind:       semanticir.PredicateFalse,
		Provenance: proofProvenance(proofArtifact("tests", semanticir.ArtifactTests)),
	})
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.Reference.Verdict != proof.VerdictVerified || result.FalsePositive.Verdict != proof.VerdictVerified || result.Fairness.Verdict != proof.VerdictNotVerified {
		t.Fatalf("constant-false suite obligations: reference=%s soundness=%s fairness=%s", result.Reference.Verdict, result.FalsePositive.Verdict, result.Fairness.Verdict)
	}
}

func TestProofAuthoritativeTestSuite(t *testing.T) {
	task := proofTask([]string{"good"}, truePredicate())
	// The suite cannot replace independently translated per-artifact predicates.
	predicate := semanticir.TestPredicate{Kind: semanticir.PredicateFalse, Provenance: proofProvenance(task.Artifacts[1].Artifact)}
	task.TestSuite.Predicate = predicate
	assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-test-ir")

	labels := proofTask([]string{"good"}, truePredicate())
	labels.Requirements[0].TestIDs = []string{"author-test-label"}
	syncProofArtifacts(labels)
	labelResult := proof.Verify(context.Background(), labels)
	if labelResult.Verdict == proof.VerdictProofBlocked {
		t.Fatalf("author-facing requirement test labels must not require advisory Task.Tests rows: %+v", labelResult.Blockers)
	}

	partial := proofTask([]string{"good"}, truePredicate())
	partial.Artifacts[1].Coverage.Status = semanticir.TranslationPartial
	partial.Artifacts[1].Coverage.TranslatedConstructs = 0
	partial.Artifacts[1].Coverage.Unsupported = []semanticir.UnsupportedConstruct{{Kind: "opaque-advisory-assertion", Provenance: proofProvenance(partial.Artifacts[1].Artifact)}}
	syncProofTestSuite(partial)
	partialResult := proof.Verify(context.Background(), partial)
	if partialResult.Verdict != proof.VerdictProofBlocked {
		t.Fatalf("partial static test translation must block, got %+v", partialResult)
	}

	staticOnly := proofTask([]string{"good"}, andPredicate(
		outcomeIn("x0", "f", assignment("x", "0"), "good"),
		outcomeIn("x1", "f", assignment("x", "1"), "good"),
	))
	assertProofVerdict(t, proof.Verify(context.Background(), staticOnly), proof.VerdictVerified)
}

func TestProofConfigurationEnvironment(t *testing.T) {
	task := proofTask([]string{"good"}, andPredicate(
		outcomeIn("x0", "f", assignment("x", "0"), "good"),
		outcomeIn("x1", "f", assignment("x", "1"), "good"),
	))
	task.Environment.Artifact = semanticir.ArtifactRef{}
	syncProofTestSuite(task)
	assertProofVerdict(t, proof.Verify(context.Background(), task), proof.VerdictVerified)
}

func TestProofObservationCompleteness(t *testing.T) {
	t.Run("missing_projection_component", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.TestSuite.ObservationCompleteness.ProjectionComponents = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-test-ir")
	})

	t.Run("stale_projection_component", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.TestSuite.ObservationCompleteness.ProjectionComponents[0].Digest = proofDigest("stale-projection")
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-test-ir")
	})

	t.Run("missing_behavior_dependency", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Tests[0].Predicate = outcomeIn("x0", "f", assignment("x", "0"), "good")
		syncProofArtifacts(task)
		task.Artifacts[1].TestProjection.Dependencies = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-test-ir")
	})
}

func TestProofValidateResult(t *testing.T) {
	task := proofTask([]string{"good"}, andPredicate(
		outcomeIn("x0", "f", assignment("x", "0"), "good"),
		outcomeIn("x1", "f", assignment("x", "1"), "good"),
	))
	result := proof.Verify(context.Background(), task)
	if err := proof.ValidateResult(task, result); err != nil {
		t.Fatalf("ValidateResult rejected canonical result: %v", err)
	}
	result.Transcript.OutcomeUniverse++
	if err := proof.ValidateResult(task, result); err == nil {
		t.Fatal("ValidateResult accepted a tampered transcript")
	}
}

func TestProofBareSemanticLabels(t *testing.T) {
	task := proofTask([]string{"good"}, andPredicate(
		outcomeIn("x0", "f", assignment("x", "0"), "good"),
		outcomeIn("x1", "f", assignment("x", "1"), "good"),
	))
	for i := range task.Domains[0].Values {
		task.Domains[0].Values[i].Value = nil
	}
	syncProofArtifacts(task)
	assertProofVerdict(t, proof.Verify(context.Background(), task), proof.VerdictVerified)
}

func TestProofSolverUnavailable(t *testing.T) {
	task := largeProofTask()
	for index, tool := range task.Environment.Tools {
		if tool.Name == "z3" {
			task.Environment.Tools = append(task.Environment.Tools[:index], task.Environment.Tools[index+1:]...)
			break
		}
	}
	assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "solver-blocked")
}

func TestProofSolverFallback(t *testing.T) {
	path, err := exec.LookPath("z3")
	if err != nil {
		t.Skip("z3 is not installed")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	versionOutput, err := exec.Command(path, "-version").Output()
	if err != nil {
		t.Fatal(err)
	}
	task := largeProofTask()
	_ = path
	_ = hash
	_ = versionOutput
	result := proof.Verify(context.Background(), task)
	assertProofVerdict(t, result, proof.VerdictNotVerified)
	if result.Transcript.Method != "z3-qf-lia" || result.Transcript.Solver == nil || len(result.Transcript.Solver.Queries) != 4 {
		t.Fatalf("solver transcript = %+v, want four frozen Z3 queries", result.Transcript)
	}
	if result.FalsePositive.Witness == nil || len(result.FalsePositive.Witness.Choices) != 8 {
		t.Fatalf("solver witness = %+v, want complete 8-component vector", result.FalsePositive.Witness)
	}
}

func largeProofTask() *semanticir.Task {
	task := proofTask([]string{"good"}, truePredicate())
	for value := 2; value < 8; value++ {
		id := strconv.Itoa(value)
		goodID := proofOutcomeAlias(task.Outcomes, "good")
		task.Domains[0].Values = append(task.Domains[0].Values, semanticir.DomainValue{ID: id, Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: int64(value)}, Provenance: proofProvenance(task.Spec)})
		task.Requirements = append(task.Requirements, semanticir.RequirementCase{
			ID: "req-" + id, OperationID: "f", Conditions: assignment("x", id), RequiredOutcomes: []string{goodID}, ForbiddenOutcomes: subtractOutcomes(task.Operations[0].OutcomeIDs, []string{goodID}),
			InstructionClauseIDs: []string{"clause-0"}, InstructionSources: []semanticir.Provenance{proofProvenance(task.Instruction)}, Evidence: []semanticir.Provenance{proofProvenance(task.Artifacts[0].Artifact)}, Provenance: proofProvenance(task.Spec),
		})
		task.CodeCases = append(task.CodeCases, semanticir.BehaviorCase{ID: "code-" + id, OperationID: "f", Conditions: assignment("x", id), Inputs: proofBehaviorInputs(assignment("x", id)), OutcomeIDs: []string{"good"}, Provenance: proofProvenance(task.Artifacts[0].Artifact)})
	}
	syncProofArtifacts(task)
	return task
}

func TestProofBlocked(t *testing.T) {
	t.Run("nil task", func(t *testing.T) {
		assertProofEngineBlocked(t, proof.Verify(context.Background(), nil), "nil-task")
	})
	t.Run("non finite domain", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Domains[0].Values = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-spec-ir")
	})
	t.Run("incomplete translation", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Artifacts[0].Coverage.Status = semanticir.TranslationPartial
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "incomplete-translation")
	})
	t.Run("zero fact coverage", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.InstructionModel.Coverage.TotalConstructs = 0
		task.InstructionModel.Coverage.TranslatedConstructs = 0
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "incomplete-translation")
	})
	t.Run("missing global predicate", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Tests[0].Predicate = semanticir.TestPredicate{}
		task.Artifacts[1].Tests[0].Predicate = semanticir.TestPredicate{}
		task.TestSuite.Predicate = semanticir.TestPredicate{}
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-test-ir")
	})
	t.Run("contradictory universe", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Constraints = []semanticir.Constraint{
			{ID: "exclude-0", OperationID: "f", Conditions: assignment("x", "0"), Reason: "unreachable zero", Provenance: proofProvenance(task.Spec)},
			{ID: "exclude-1", OperationID: "f", Conditions: assignment("x", "1"), Reason: "unreachable one", Provenance: proofProvenance(task.Spec)},
		}
		syncProofArtifacts(task)
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "contradictory-universe")
	})
	t.Run("flattened model conflict", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.CodeCases[0].OutcomeIDs = []string{"bad"}
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "conflicting-code-model")
	})
	t.Run("unsupported invariant", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Invariants = []semanticir.Invariant{{
			ID: "state", Predicate: semanticir.Expression{Kind: semanticir.ExprCall, Type: semanticir.TypeBool, Name: "opaque", Provenance: proofProvenance(task.Spec)}, Provenance: proofProvenance(task.Spec),
		}}
		task.Requirements[0].InvariantIDs = []string{"state"}
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-spec-ir")
	})
	t.Run("nondeterministic code row", func(t *testing.T) {
		task := proofTask([]string{"alt", "good"}, truePredicate())
		task.CodeCases[0].OutcomeIDs = []string{"alt", "good"}
		syncProofArtifacts(task)
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("missing compiler grounding", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Artifacts[0].CompilerEvidence = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("fabricated exhaustive effect observation", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Outcomes[2].Effects = []semanticir.Effect{{ID: "claimed-write", Kind: semanticir.EffectWrite, Target: "audit", Provenance: proofProvenance(task.Spec)}}
		syncProofArtifacts(task)
		code := &task.Artifacts[0]
		code.CompilerEvidence = nil
		code.ExhaustiveEvidence = []semanticir.ExhaustiveExecutionEvidence{{ID: "forged-effect", Complete: true, Provenance: proofProvenance(code.Artifact)}}
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("missing patch scope closure", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Artifacts[0].ScopeClosure = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("omitted impacted operation owner", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Artifacts[0].ScopeClosure.OperationOwners = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("missing typed compiler graph", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Artifacts[0].CompilerEvidence[0].SemanticGraph = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("missing compiler graph operation", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Artifacts[0].CompilerEvidence[0].SemanticGraph.Operations = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("decoder does not bind changed semantic node", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		graph := task.Artifacts[0].CompilerEvidence[0].SemanticGraph
		graph.Nodes[len(graph.Nodes)-1].Message = "tampered"
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("ambient compiler derivation environment", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.Artifacts[0].CompilerEvidence[0].SemanticGraph.DerivationSteps[0].ClearEnvironment = false
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("missing graph terminal", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		root := &task.Artifacts[0].CompilerEvidence[0].SemanticGraph.Operations[0]
		root.TerminalNodeIDs = root.TerminalNodeIDs[:1]
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("tampered graph terminal outcome", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		graph := task.Artifacts[0].CompilerEvidence[0].SemanticGraph
		for index := range graph.Nodes {
			if graph.Nodes[index].Kind == semanticir.CompilerNodeReturn {
				graph.Nodes[index].Kind = semanticir.CompilerNodeSuccess
				break
			}
		}
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("tampered graph control edge", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		graph := task.Artifacts[0].CompilerEvidence[0].SemanticGraph
		graph.Edges[0].ToBlockID = graph.Edges[0].FromBlockID
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-reference-ir")
	})
	t.Run("missing authoritative test suite", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.TestSuite = nil
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-test-ir")
	})
	t.Run("obsolete inline vector evidence", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		task.TestSuite.AcceptedVectorsDigest = proofDigest("stale-accepted")
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-test-ir")
	})
	t.Run("incomplete full cross-check", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		vectors, _, accepted, vectorDigest, acceptedDigest := proofTestVectorEvidence(task, task.TestSuite.Predicate)
		vectors = vectors[:len(vectors)-1]
		vectorDigest, acceptedDigest, _ = semanticir.TestVectorDigests(vectors)
		accepted--
		task.TestSuite.CrossCheck = &semanticir.TestCrossCheckEvidence{
			Full: true, Vectors: vectors, AcceptedVectorCount: accepted, VectorEvidenceDigest: vectorDigest, AcceptedVectorsDigest: acceptedDigest,
			Repetitions: 2, RunDigests: []string{vectorDigest, vectorDigest}, Provenance: proofProvenance(task.Artifacts[1].Artifact),
		}
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "invalid-test-ir")
	})
	t.Run("cross-check disagrees with predicate", func(t *testing.T) {
		task := proofTask([]string{"good"}, truePredicate())
		vectors, _, accepted, _, _ := proofTestVectorEvidence(task, task.TestSuite.Predicate)
		vectors[0].Accepted = false
		vectorDigest, acceptedDigest, err := semanticir.TestVectorDigests(vectors)
		if err != nil {
			t.Fatal(err)
		}
		task.TestSuite.CrossCheck = &semanticir.TestCrossCheckEvidence{
			Vectors: vectors, AcceptedVectorCount: accepted - 1, VectorEvidenceDigest: vectorDigest, AcceptedVectorsDigest: acceptedDigest,
			Repetitions: 2, RunDigests: []string{vectorDigest, vectorDigest}, Provenance: proofProvenance(task.Artifacts[1].Artifact),
		}
		assertProofEngineBlocked(t, proof.Verify(context.Background(), task), "stale-test-cross-check")
	})
}

func proofTask(required []string, predicate semanticir.TestPredicate) *semanticir.Task {
	instruction := proofArtifact("instruction", semanticir.ArtifactInstruction)
	spec := proofArtifact("spec", semanticir.ArtifactSpec)
	code := proofArtifact("code", semanticir.ArtifactCode)
	tests := proofArtifact("tests", semanticir.ArtifactTests)
	configuration := proofArtifact("configuration", semanticir.ArtifactConfiguration)
	environment := proofArtifact("environment", semanticir.ArtifactEnvironment)
	domain := semanticir.Domain{ID: "x", Type: semanticir.TypeInteger, Provenance: proofProvenance(spec), Values: []semanticir.DomainValue{
		{ID: "0", Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 0}, Provenance: proofProvenance(spec)},
		{ID: "1", Value: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}, Provenance: proofProvenance(spec)},
	}}
	outcomes := []semanticir.ObservableOutcome{
		{Kind: semanticir.OutcomeReturn, OperationID: "f", Value: &semanticir.Literal{Type: semanticir.TypeString, String: "alt"}, Provenance: proofProvenance(spec)},
		{Kind: semanticir.OutcomeReturn, OperationID: "f", Value: &semanticir.Literal{Type: semanticir.TypeString, String: "bad"}, Provenance: proofProvenance(spec)},
		{Kind: semanticir.OutcomeReturn, OperationID: "f", Value: &semanticir.Literal{Type: semanticir.TypeString, String: "good"}, Provenance: proofProvenance(spec)},
		semanticir.OtherOutcome("f", proofProvenance(spec)),
	}
	for index := 0; index < 3; index++ {
		outcomes[index].ID = semanticir.OutcomeID(outcomes[index])
	}
	allOutcomeIDs := []string{outcomes[0].ID, outcomes[1].ID, outcomes[2].ID, outcomes[3].ID}
	required = proofCanonicalOutcomeIDs(outcomes, required)
	predicate = proofCanonicalPredicateOutcomes(outcomes, predicate)
	forbidden := subtractOutcomes(allOutcomeIDs, required)
	operation := semanticir.Operation{
		ID: "f", Kind: semanticir.OperationFunction, DomainIDs: []string{"x"}, OutcomeIDs: allOutcomeIDs,
		Inputs:     []semanticir.Variable{{Name: "x", Type: semanticir.TypeInteger, DomainID: "x", Provenance: proofProvenance(spec)}},
		Provenance: proofProvenance(spec),
	}
	for index := range domain.Values {
		literal := *domain.Values[index].Value
		membership := proofEqualityMembership(spec, "x", literal)
		domain.Values[index].Groundings = []semanticir.GroundingAxiom{{
			OperationID: operation.ID, Kind: semanticir.GroundingMembership, Membership: &membership,
			ConcreteWitness: map[string]semanticir.Literal{"x": literal}, Provenance: proofProvenance(spec),
		}}
	}
	clauses := []semanticir.InstructionClause{
		{ID: "clause-0", Span: proofProvenance(instruction).Location, SliceDigest: proofDigest("slice-0"), Provenance: proofProvenance(instruction)},
		{ID: "clause-1", Span: proofProvenance(instruction).Location, SliceDigest: proofDigest("slice-1"), Provenance: proofProvenance(instruction)},
	}
	requirements := make([]semanticir.RequirementCase, 2)
	codeCases := make([]semanticir.BehaviorCase, 2)
	for i, value := range []string{"0", "1"} {
		requirements[i] = semanticir.RequirementCase{
			ID: "req-" + value, OperationID: "f", Conditions: assignment("x", value),
			RequiredOutcomes: append([]string(nil), required...), ForbiddenOutcomes: append([]string(nil), forbidden...),
			InstructionClauseIDs: []string{"clause-" + value}, InstructionSources: []semanticir.Provenance{proofProvenance(instruction)},
			Evidence: []semanticir.Provenance{proofProvenance(code)}, Provenance: proofProvenance(spec),
		}
		codeCases[i] = semanticir.BehaviorCase{ID: "code-" + value, OperationID: "f", Conditions: assignment("x", value), Inputs: proofBehaviorInputs(assignment("x", value)), OutcomeIDs: []string{proofOutcomeAlias(outcomes, "good")}, Provenance: proofProvenance(code)}
	}
	testModels := []semanticir.TestModel{{ID: "suite", Predicate: predicate, Provenance: proofProvenance(tests)}}
	task := &semanticir.Task{
		ID: "proof-fixture", Instruction: instruction, Spec: spec,
		InstructionModel: semanticir.InstructionModel{Artifact: instruction, Clauses: clauses, Coverage: proofCoverage(instruction)},
		Environment: &semanticir.EnvironmentModel{
			Artifact: configuration, Configuration: configuration, SourceArtifacts: []semanticir.ArtifactRef{environment}, Identity: "fixture-linux", ConfigDigest: proofDigest("config"),
			Tools: []semanticir.ToolRef{proofTool("runner"), proofTool("python-frontend"), proofTool("python-compiler"), proofTool("finite-prover")}, Commands: proofCommands(configuration), Coverage: proofCoverage(configuration), Provenance: proofProvenance(configuration),
		},
		Domains: []semanticir.Domain{domain}, Operations: []semanticir.Operation{operation}, Outcomes: outcomes,
		Groundings: []semanticir.AssignmentGrounding{
			{ID: semanticir.AssignmentGroundingID("f", assignment("x", "0")), OperationID: "f", Conditions: assignment("x", "0"), Inputs: proofBehaviorInputs(assignment("x", "0")), Provenance: proofProvenance(spec)},
			{ID: semanticir.AssignmentGroundingID("f", assignment("x", "1")), OperationID: "f", Conditions: assignment("x", "1"), Inputs: proofBehaviorInputs(assignment("x", "1")), Provenance: proofProvenance(spec)},
		},
		Requirements: requirements, CodeCases: codeCases, Tests: testModels, Provenance: proofProvenance(spec),
		Coverage: []semanticir.TranslationCoverage{proofCoverage(spec)},
	}
	task.Artifacts = []semanticir.ArtifactModel{
		{Artifact: code, Kind: semanticir.ArtifactCode, Language: semanticir.LanguagePython, Coverage: proofCoverage(code), Translator: proofTool("python-frontend")},
		{Artifact: tests, Kind: semanticir.ArtifactTests, Language: semanticir.LanguagePython, Coverage: proofCoverage(tests), Translator: proofTool("python-frontend")},
	}
	syncProofArtifacts(task)
	return task
}

func syncProofArtifacts(task *semanticir.Task) {
	syncProofOutcomeAliases(task)
	syncProofGroundingFixture(task)
	refreshProofSpecIR(task)
	for i := range task.Artifacts {
		task.Artifacts[i].Domains = append([]semanticir.Domain(nil), task.Domains...)
		task.Artifacts[i].Constraints = append([]semanticir.Constraint(nil), task.Constraints...)
		task.Artifacts[i].Groundings = append([]semanticir.AssignmentGrounding(nil), task.Groundings...)
		task.Artifacts[i].Outcomes = append([]semanticir.ObservableOutcome(nil), task.Outcomes...)
		switch task.Artifacts[i].Kind {
		case semanticir.ArtifactCode:
			task.Artifacts[i].Operations = make([]semanticir.Operation, len(task.Operations))
			for j := range task.Operations {
				task.Artifacts[i].Operations[j] = proofModelOperation(task.Operations[j], task.Artifacts[i].Artifact)
			}
			task.Artifacts[i].Cases = append([]semanticir.BehaviorCase(nil), task.CodeCases...)
			task.Artifacts[i].RawReferenceCases = proofRawReferenceCases(task, task.CodeCases, task.Artifacts[i].Artifact)
			task.Artifacts[i].CompilerEvidence = proofCompilerEvidence(task, task.Artifacts[i].Artifact, task.Artifacts[i].Language)
			task.Artifacts[i].ScopeClosure = proofScopeClosure(task, &task.Artifacts[i])
		case semanticir.ArtifactTests:
			task.Artifacts[i].Tests = append([]semanticir.TestModel(nil), task.Tests...)
			task.Artifacts[i].TestProjection, task.Artifacts[i].RunnerSelection = proofTestArtifactEvidence(task, &task.Artifacts[i])
		}
	}
	syncProofTestSuite(task)
}

func proofRawReferenceCases(task *semanticir.Task, cases []semanticir.BehaviorCase, artifact semanticir.ArtifactRef) []semanticir.RawReferenceCase {
	byID := make(map[string]semanticir.ObservableOutcome, len(task.Outcomes))
	for _, outcome := range task.Outcomes {
		byID[outcome.ID] = outcome
	}
	result := make([]semanticir.RawReferenceCase, 0, len(cases))
	for _, item := range cases {
		raw := semanticir.RawReferenceCase{
			ID: item.ID, OperationID: item.OperationID, Conditions: assignmentCopy(item.Conditions),
			Inputs: assignmentInputsCopy(item.Inputs), Provenance: proofProvenance(artifact),
		}
		for _, outcomeID := range item.OutcomeIDs {
			outcome, exists := byID[outcomeID]
			if !exists {
				continue
			}
			trace := semanticir.RawOutcomeTrace{
				Kind: outcome.Kind, Value: outcome.Value, ExceptionType: outcome.ExceptionType, Message: outcome.Message,
			}
			if outcome.Kind == semanticir.OutcomeOther {
				literal := semanticir.Literal{Type: semanticir.TypeString, String: "__ray_proof_fixture_other__"}
				trace = semanticir.RawOutcomeTrace{Kind: semanticir.OutcomeReturn, Value: &literal}
			}
			for _, effect := range outcome.Effects {
				var value *semanticir.Literal
				if effect.Value != nil && effect.Value.Kind == semanticir.ExprLiteral && effect.Value.Literal != nil {
					literal := *effect.Value.Literal
					value = &literal
				}
				trace.Effects = append(trace.Effects, semanticir.RawEffectTrace{Kind: effect.Kind, Target: effect.Target, Value: value})
			}
			raw.Outcomes = append(raw.Outcomes, trace)
		}
		result = append(result, raw)
	}
	return result
}

func refreshProofSpecIR(task *semanticir.Task) {
	digest, err := semanticir.CanonicalSpecIRDigest(task)
	if err != nil {
		panic(err)
	}
	task.SpecIRDigest = digest
}

func syncProofOutcomeAliases(task *semanticir.Task) {
	replacements := map[string]string{}
	for index := range task.Outcomes {
		outcome := &task.Outcomes[index]
		oldID := outcome.ID
		outcome.ID = semanticir.OutcomeID(*outcome)
		if oldID != "" {
			replacements[oldID] = outcome.ID
		}
		if outcome.Kind == semanticir.OutcomeReturn && outcome.Value != nil && outcome.Value.Type == semanticir.TypeString {
			replacements[outcome.Value.String] = outcome.ID
		}
	}
	replace := func(values []string) {
		for index, value := range values {
			if canonical, exists := replacements[value]; exists {
				values[index] = canonical
			}
		}
	}
	for index := range task.Operations {
		replace(task.Operations[index].OutcomeIDs)
	}
	for index := range task.Requirements {
		replace(task.Requirements[index].RequiredOutcomes)
		replace(task.Requirements[index].ForbiddenOutcomes)
	}
	for index := range task.CodeCases {
		replace(task.CodeCases[index].OutcomeIDs)
	}
	var rewritePredicate func(*semanticir.TestPredicate)
	rewritePredicate = func(predicate *semanticir.TestPredicate) {
		if predicate == nil {
			return
		}
		if predicate.Observe != nil {
			replace(predicate.Observe.OutcomeIDs)
		}
		for index := range predicate.Children {
			rewritePredicate(&predicate.Children[index])
		}
	}
	for index := range task.Tests {
		replace(task.Tests[index].AcceptedOutcomes)
		rewritePredicate(&task.Tests[index].Predicate)
	}
}

func proofCanonicalOutcomeIDs(outcomes []semanticir.ObservableOutcome, values []string) []string {
	result := append([]string(nil), values...)
	for index, value := range result {
		result[index] = proofOutcomeAlias(outcomes, value)
	}
	return result
}

func proofCanonicalPredicateOutcomes(outcomes []semanticir.ObservableOutcome, predicate semanticir.TestPredicate) semanticir.TestPredicate {
	if predicate.Observe != nil {
		predicate.Observe.OutcomeIDs = proofCanonicalOutcomeIDs(outcomes, predicate.Observe.OutcomeIDs)
	}
	for index := range predicate.Children {
		predicate.Children[index] = proofCanonicalPredicateOutcomes(outcomes, predicate.Children[index])
	}
	return predicate
}

func proofOutcomeAlias(outcomes []semanticir.ObservableOutcome, value string) string {
	for _, outcome := range outcomes {
		if outcome.ID == value || outcome.Kind == semanticir.OutcomeReturn && outcome.Value != nil && outcome.Value.Type == semanticir.TypeString && outcome.Value.String == value {
			return outcome.ID
		}
	}
	return value
}

func syncProofGroundingFixture(task *semanticir.Task) {
	for operationIndex := range task.Operations {
		operation := &task.Operations[operationIndex]
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		for _, domainID := range operation.DomainIDs {
			for domainIndex := range task.Domains {
				domain := &task.Domains[domainIndex]
				if domain.ID != domainID {
					continue
				}
				for valueIndex := range domain.Values {
					value := &domain.Values[valueIndex]
					if _, exists := value.GroundingFor(operation.ID); exists || value.Value == nil {
						continue
					}
					inputName := domainID
					for _, input := range operation.Inputs {
						if input.DomainID == domainID {
							inputName = input.Name
							break
						}
					}
					membership := proofEqualityMembership(task.Spec, inputName, *value.Value)
					value.Groundings = append(value.Groundings, semanticir.GroundingAxiom{
						OperationID: operation.ID, Kind: semanticir.GroundingMembership, Membership: &membership,
						ConcreteWitness: map[string]semanticir.Literal{inputName: *value.Value}, Provenance: proofProvenance(task.Spec),
					})
				}
			}
		}
	}
	for index := range task.CodeCases {
		codeCase := &task.CodeCases[index]
		if codeCase.Inputs != nil {
			continue
		}
		operation, exists := findProofOperation(task.Operations, codeCase.OperationID)
		if !exists {
			continue
		}
		if inputs, exact := semanticir.ExactGroundingInputs(operation, task.Domains, codeCase.Conditions); exact {
			codeCase.Inputs = assignmentInputsCopy(inputs)
		}
	}
	task.Groundings = nil
	seen := map[string]bool{}
	for requirementIndex := range task.Requirements {
		requirement := &task.Requirements[requirementIndex]
		operation, exists := findProofOperation(task.Operations, requirement.OperationID)
		if !exists {
			continue
		}
		inputs, exact := semanticir.ExactGroundingInputs(operation, task.Domains, requirement.Conditions)
		if !exact {
			continue
		}
		id := semanticir.AssignmentGroundingID(operation.ID, requirement.Conditions)
		requirement.GroundingID = id
		if seen[id] {
			continue
		}
		seen[id] = true
		task.Groundings = append(task.Groundings, semanticir.AssignmentGrounding{
			ID: id, OperationID: operation.ID, Conditions: assignmentCopy(requirement.Conditions), Inputs: assignmentInputsCopy(inputs), Provenance: proofProvenance(task.Spec),
		})
	}
}

func proofScopeClosure(task *semanticir.Task, artifact *semanticir.ArtifactModel) *semanticir.ScopeClosureEvidence {
	provenance := proofProvenance(artifact.Artifact)
	compiler := artifact.Translator
	prover := proofTool("finite-prover")
	irDigest := proofDigest(artifact.Artifact.ID + "/scope-ir")
	evidence := &semanticir.ScopeClosureEvidence{
		SourceArtifacts: []semanticir.ArtifactRef{artifact.Artifact}, WorkspaceTreeDigest: proofWorkspaceDigest(),
		Compiler: compiler, Prover: prover, CompilerIRDigest: irDigest,
		Completeness: semanticir.ProofProved, Complete: true, Provenance: provenance,
	}
	for index, operation := range artifact.Operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		declarationID := "declaration-" + operation.ID
		evidence.ChangedRanges = append(evidence.ChangedRanges, semanticir.ChangedSourceRange{
			ArtifactID: artifact.Artifact.ID, Path: artifact.Artifact.Path, StartLine: 1, EndLine: 1,
			SliceDigest: proofDigest("changed-" + operation.ID), Provenance: provenance,
		})
		evidence.Declarations = append(evidence.Declarations, semanticir.CompilerDeclaration{
			ID: declarationID, QualifiedName: operation.ID, Artifact: artifact.Artifact,
			Location:        semanticir.SourceLocation{Path: artifact.Artifact.Path, StartLine: 1, StartColumn: index + 1, EndLine: 1, EndColumn: index + 1},
			CompilerNodeIDs: []string{"scope-node-" + operation.ID}, Changed: true, Provenance: provenance,
		})
		evidence.ImpactedDeclarationIDs = append(evidence.ImpactedDeclarationIDs, declarationID)
		evidence.OperationOwners = append(evidence.OperationOwners, semanticir.OperationOwner{OperationID: operation.ID, DeclarationID: declarationID})
	}
	sort.Strings(evidence.ImpactedDeclarationIDs)
	graphDigest, err := semanticir.ScopeClosureGraphDigest(*evidence)
	if err != nil {
		panic(err)
	}
	sourceDigest, err := semanticir.Digest(evidence.SourceArtifacts)
	if err != nil {
		panic(err)
	}
	declarations := []byte("(declare-const ray_scope_omitted Bool)")
	scopeFormula := []byte("(and ray_scope_omitted (not ray_scope_omitted))")
	scope := semanticir.CompilerPredicate{
		Logic: semanticir.ProofLogicSMTLIB2, Declarations: declarations, DeclarationsDigest: semanticir.DigestBytes(declarations),
		Formula: scopeFormula, FormulaDigest: semanticir.DigestBytes(scopeFormula), Tool: compiler, IRDigest: irDigest,
		CompilerNodeIDs: []string{"scope-omission-query"},
	}
	context := semanticir.CompilerProofContext{
		SourceDigest: sourceDigest, WorkspaceTreeDigest: evidence.WorkspaceTreeDigest,
		EmittedIRDigest: irDigest, HarnessDigest: graphDigest, Compiler: compiler,
	}
	claim := semanticir.NewProofClaim(semanticir.ClaimScopeClosure, context, scope, nil, nil)
	evidence.CompletenessProof = proofReplayableProof(claim, semanticir.SolverUNSAT, prover, proofReplayEnvironmentDigest())
	return evidence
}

func proofTestArtifactEvidence(task *semanticir.Task, artifact *semanticir.ArtifactModel) (*semanticir.TestObservationProjection, *semanticir.RunnerSelectionEvidence) {
	provenance := proofProvenance(artifact.Artifact)
	translator := artifact.Translator
	prover := proofTool("finite-prover")
	irDigest := proofDigest(artifact.Artifact.ID + "/test-ir")
	predicate := semanticir.StaticTestPredicate(artifact.Tests, provenance)
	predicateDigest, err := semanticir.Digest(predicate)
	if err != nil {
		panic(err)
	}
	modelBytes := []byte("sat\n")
	modelDigest, err := semanticir.Digest(artifact.Tests)
	if err != nil {
		panic(err)
	}
	query := []byte("(set-logic ALL)\n(check-sat)\n")
	step := semanticir.ProbeStep{
		ID: "derive-test-ir", Kind: semanticir.ProbeStepRun, Tool: prover,
		Argv: []string{"-in", "-smt2"}, Stdin: query, StdinDigest: semanticir.DigestBytes(query),
		WorkingDirectory: proofWorkspaceRoot(), Environment: []semanticir.EnvironmentVariable{}, EnvironmentDigest: proofReplayEnvironmentDigest(),
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 1000, ExpectedExitCode: 0,
		ExpectedStdoutDigest: semanticir.DigestBytes([]byte("sat\n")), ExpectedStderrDigest: semanticir.DigestBytes(nil), ExpectedSignalDigest: semanticir.DigestBytes(nil),
		SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: provenance,
	}
	projection := &semanticir.TestObservationProjection{
		Source: artifact.Artifact, PredicateDigest: predicateDigest, Complete: true, Provenance: provenance,
		Derivation: semanticir.CompilerDerivationEvidence{
			SourceDigest: artifact.Artifact.Digest, WorkspaceTreeDigest: task.Environment.Commands[2].TreeDigest,
			Tool: translator, IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: irDigest,
			Steps: []semanticir.ProbeStep{step}, Output: modelBytes, OutputDigest: semanticir.DigestBytes(modelBytes), DecodedModelDigest: modelDigest, Complete: true,
		},
	}
	for testIndex, test := range artifact.Tests {
		projection.TestIDs = append(projection.TestIDs, test.ID)
		constructID := "construct-" + test.ID
		projection.Constructs = append(projection.Constructs, semanticir.TestConstructEvidence{
			ID: constructID, ArtifactID: artifact.Artifact.ID, Kind: semanticir.TestConstructAssertion,
			Digest: proofDigest(constructID), IRKind: semanticir.CompilerIRCPythonBytecode, IRDigest: irDigest,
			Tool: translator, CompilerNodeIDs: []string{"construct-node-" + test.ID}, Provenance: provenance,
		})
		rootID := proofProjectionNodes(&projection.Nodes, test.Predicate, constructID, fmt.Sprintf("test-%d", testIndex))
		projection.PassRoots = append(projection.PassRoots, semanticir.TestPassRoot{TestID: test.ID, NodeID: rootID, CompilerNodeIDs: []string{"pass-root-" + test.ID}})
		for dependencyIndex, behavior := range proofPredicateBehaviors(test.Predicate) {
			projection.Dependencies = append(projection.Dependencies, semanticir.TestBehaviorDependency{
				ConstructID: constructID, Kind: semanticir.TestDependencyCall, Behavior: behavior, Inputs: assignmentInputsCopy(behavior.Inputs),
				CompilerNodeIDs: []string{fmt.Sprintf("dependency-node-%s-%d", test.ID, dependencyIndex)}, Provenance: provenance,
			})
		}
	}
	quantified := map[string]bool{}
	for _, dependency := range projection.Dependencies {
		category := semanticir.BehaviorRef{OperationID: dependency.Behavior.OperationID, Conditions: assignmentCopy(dependency.Behavior.Conditions), Provenance: provenance}
		key := category.OperationID + canonicalTestAssignment(category.Conditions)
		if quantified[key] {
			continue
		}
		quantified[key] = true
		inputs := []map[string]semanticir.Literal{assignmentInputsCopy(dependency.Behavior.Inputs)}
		digest, digestErr := semanticir.TestConcreteInputsDigest(inputs)
		if digestErr != nil {
			panic(digestErr)
		}
		projection.Quantification = append(projection.Quantification, semanticir.TestQuantificationEvidence{
			Behavior: category, Kind: semanticir.TestQuantificationSingleton, ConcreteInputs: inputs,
			ConcreteInputsDigest: digest, Result: semanticir.ProofProved, Provenance: provenance,
		})
	}
	runner := &semanticir.RunnerSelectionEvidence{
		TestIDs: append([]string(nil), projection.TestIDs...), PredicateDigest: predicateDigest,
		Configuration: environmentConfigurationForTest(task.Environment), Verifier: proofTool("runner"), Command: task.Environment.Commands[2],
		ConjunctivePass: true, Complete: true, Provenance: proofProvenance(environmentConfigurationForTest(task.Environment)),
	}
	return projection, runner
}

func proofPredicateBehaviors(predicate semanticir.TestPredicate) []semanticir.BehaviorRef {
	seen := map[string]bool{}
	var values []semanticir.BehaviorRef
	add := func(value *semanticir.BehaviorRef) {
		if value == nil {
			return
		}
		key := semanticir.BehaviorRefKey(*value)
		if !seen[key] {
			seen[key] = true
			values = append(values, *value)
		}
	}
	var walk func(semanticir.TestPredicate)
	walk = func(value semanticir.TestPredicate) {
		if value.Observe != nil {
			add(&value.Observe.Behavior)
		}
		add(value.Left)
		add(value.Right)
		for _, child := range value.Children {
			walk(child)
		}
	}
	walk(predicate)
	return values
}

func proofProjectionNodes(nodes *[]semanticir.TestProjectionNode, predicate semanticir.TestPredicate, constructID, prefix string) string {
	id := prefix + "-node-" + strconv.Itoa(len(*nodes))
	rootIndex := len(*nodes)
	node := semanticir.TestProjectionNode{
		ID: id, Kind: predicate.Kind, Observe: predicate.Observe, Left: predicate.Left, Right: predicate.Right,
		CompilerNodeIDs: []string{"compiler-" + id}, ConstructIDs: []string{constructID}, Provenance: predicate.Provenance,
	}
	*nodes = append(*nodes, node)
	for index, child := range predicate.Children {
		childID := proofProjectionNodes(nodes, child, constructID, fmt.Sprintf("%s-%d", prefix, index))
		(*nodes)[rootIndex].Children = append((*nodes)[rootIndex].Children, childID)
	}
	return id
}

func syncProofTestSuite(task *semanticir.Task) {
	predicate := semanticir.StaticTestPredicate(task.Tests, proofProvenance(proofArtifact("tests", semanticir.ArtifactTests)))
	_, vectorCount, _, _, _ := proofTestVectorEvidence(task, predicate)
	sources := []semanticir.ArtifactRef{environmentConfigurationForTest(task.Environment)}
	sources = append(sources, task.Environment.SourceArtifacts...)
	var models []semanticir.ArtifactModelDigest
	var testArtifacts []semanticir.ArtifactModel
	for _, artifact := range task.Artifacts {
		if artifact.Kind != semanticir.ArtifactTests {
			continue
		}
		testArtifacts = append(testArtifacts, artifact)
		sources = append(sources, artifact.Artifact)
		digest, err := semanticir.Digest(artifact)
		if err != nil {
			panic(err)
		}
		models = append(models, semanticir.ArtifactModelDigest{ArtifactID: artifact.Artifact.ID, Digest: digest})
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].ID != sources[j].ID {
			return sources[i].ID < sources[j].ID
		}
		return sources[i].Path < sources[j].Path
	})
	sort.Slice(models, func(i, j int) bool { return models[i].ArtifactID < models[j].ArtifactID })
	evidence := make([]semanticir.Provenance, len(sources))
	for i := range sources {
		evidence[i] = proofProvenance(sources[i])
	}
	suite := &semanticir.TestSuiteModel{
		SourceArtifacts: sources, SourceModels: models, Predicate: predicate, Verifier: proofTool("runner"), Execution: task.Environment.Commands[2],
		VectorCount: vectorCount, Coverage: proofCoverage(environmentConfigurationForTest(task.Environment)), Evidence: evidence,
	}
	composition, err := testir.ComposeRunner(testArtifacts, sources, suite.Verifier, suite.Execution, suite.Predicate.Provenance)
	if err != nil {
		// Invalid-test adversaries deliberately create input that cannot be
		// composed. Preserve the incomplete suite so production validation can
		// report PROOF BLOCKED instead of letting a fixture helper panic.
		task.TestSuite = suite
		return
	}
	suite.RunnerComposition = composition
	suite.ObservationCompleteness = proofTestObservationCompleteness(task, suite)
	task.TestSuite = suite
}

func proofTestObservationCompleteness(task *semanticir.Task, suite *semanticir.TestSuiteModel) semanticir.TestObservationCompleteness {
	tests := proofArtifact("tests", semanticir.ArtifactTests)
	provenance := proofProvenance(tests)
	predicateDigest, err := semanticir.Digest(suite.Predicate)
	if err != nil {
		panic(err)
	}
	var components []semanticir.ArtifactModelDigest
	var constructs []semanticir.TestConstructEvidence
	for _, artifact := range task.Artifacts {
		if artifact.Kind != semanticir.ArtifactTests || artifact.TestProjection == nil {
			continue
		}
		digest, err := semanticir.TestProjectionGraphDigest(*artifact.TestProjection)
		if err != nil {
			panic(err)
		}
		components = append(components, semanticir.ArtifactModelDigest{ArtifactID: artifact.Artifact.ID, Digest: digest})
		constructs = append(constructs, artifact.TestProjection.Constructs...)
	}
	sort.Slice(components, func(i, j int) bool { return components[i].ArtifactID < components[j].ArtifactID })
	compositionDigest, err := semanticir.Digest(components)
	if err != nil {
		panic(err)
	}
	harnessDigest, err := semanticir.Digest(suite.Execution)
	if err != nil {
		panic(err)
	}
	return semanticir.TestObservationCompleteness{
		ProjectionComponents: components, SourceModels: append([]semanticir.ArtifactModelDigest(nil), suite.SourceModels...), StaticPredicateDigest: predicateDigest,
		IRKind: semanticir.CompilerIRCPythonBytecode, Constructs: constructs,
		ObservationIRDigest: compositionDigest, HarnessDigest: harnessDigest, Prover: proofTool("finite-prover"),
		Result: semanticir.ProofProved, ProofDigest: compositionDigest, Provenance: provenance,
	}
}

type proofFixtureCase struct {
	behavior semanticir.BehaviorRef
	outcomes []string
}

func proofTestVectorEvidence(task *semanticir.Task, predicate semanticir.TestPredicate) ([]semanticir.TestVectorResult, uint64, uint64, string, string) {
	operationOutcomes := make(map[string][]string)
	for _, operation := range task.Operations {
		operationOutcomes[operation.ID] = append([]string(nil), operation.OutcomeIDs...)
		sort.Strings(operationOutcomes[operation.ID])
	}
	cases := make([]proofFixtureCase, 0, len(task.CodeCases))
	for _, codeCase := range task.CodeCases {
		operation, found := findProofOperation(task.Operations, codeCase.OperationID)
		if !found {
			panic("missing fixture operation")
		}
		inputs, exact := semanticir.ExactGroundingInputs(operation, task.Domains, codeCase.Conditions)
		if !exact {
			panic("proof fixture category is not an exact point")
		}
		cases = append(cases, proofFixtureCase{behavior: semanticir.BehaviorRef{OperationID: codeCase.OperationID, Conditions: assignmentCopy(codeCase.Conditions), Inputs: assignmentInputsCopy(inputs), Provenance: codeCase.Provenance}, outcomes: operationOutcomes[codeCase.OperationID]})
	}
	sort.Slice(cases, func(i, j int) bool {
		left := cases[i].behavior.OperationID + canonicalTestAssignment(cases[i].behavior.Conditions)
		right := cases[j].behavior.OperationID + canonicalTestAssignment(cases[j].behavior.Conditions)
		return left < right
	})
	vectorCount := uint64(1)
	for _, item := range cases {
		vectorCount *= uint64(len(item.outcomes))
	}
	choices := make([]semanticir.BehaviorChoice, len(cases))
	var results []semanticir.TestVectorResult
	var walk func(int)
	walk = func(index int) {
		if index == len(cases) {
			copyChoices := append([]semanticir.BehaviorChoice(nil), choices...)
			results = append(results, semanticir.TestVectorResult{Choices: copyChoices, Accepted: evaluateProofPredicate(task, predicate, copyChoices)})
			return
		}
		for _, outcomeID := range cases[index].outcomes {
			choices[index] = semanticir.BehaviorChoice{Behavior: cases[index].behavior, OutcomeID: outcomeID}
			walk(index + 1)
		}
	}
	walk(0)
	var accepted uint64
	for _, result := range results {
		if result.Accepted {
			accepted++
		}
	}
	vectorDigest, acceptedDigest, err := semanticir.TestVectorDigests(results)
	if err != nil {
		panic(err)
	}
	return results, vectorCount, accepted, vectorDigest, acceptedDigest
}

func evaluateProofPredicate(task *semanticir.Task, predicate semanticir.TestPredicate, choices []semanticir.BehaviorChoice) bool {
	result, err := proof.EvaluateTestPredicate(task, predicate, choices)
	if err != nil {
		return false
	}
	return result
}

func canonicalTestAssignment(value semanticir.Assignment) string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(value[key])
		builder.WriteByte(';')
	}
	return builder.String()
}

func assignmentCopy(value semanticir.Assignment) semanticir.Assignment {
	result := make(semanticir.Assignment, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func containsTestString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func proofCompilerEvidence(task *semanticir.Task, artifact semanticir.ArtifactRef, language semanticir.Language) []semanticir.CompilerEvidence {
	provenance := proofProvenance(artifact)
	compiler := proofTool("python-compiler")
	prover := proofTool("finite-prover")
	environmentDigest := proofReplayEnvironmentDigest()
	record := semanticir.CompilerEvidence{
		ID: "compiler-" + artifact.ID, Method: semanticir.CompilerEvidenceModelChecker, FormulaDerivationDigest: proofDigest(artifact.ID + "/formula-derivation"),
		Tool: compiler, Prover: prover, SourceDigest: artifact.Digest,
		WorkspaceTreeDigest: proofWorkspaceDigest(), Argv: []string{compiler.Path, "--emit-ir", artifact.Path}, EnvironmentDigest: environmentDigest,
		EmittedIRDigest: proofDigest(artifact.ID + "/ir"), HarnessDigest: proofDigest(artifact.ID + "/harness"), TotalConstructs: 1, TranslatedConstructs: 1,
		Provenance: provenance,
	}
	record.IRKind = map[semanticir.Language]semanticir.CompilerIRKind{
		semanticir.LanguagePython: semanticir.CompilerIRCPythonBytecode,
		semanticir.LanguageRust:   semanticir.CompilerIRRustMIR,
		semanticir.LanguageCPP:    semanticir.CompilerIRLLVM,
	}[language]
	context := semanticir.CompilerProofContext{
		SourceDigest: record.SourceDigest, WorkspaceTreeDigest: record.WorkspaceTreeDigest,
		EmittedIRDigest: record.EmittedIRDigest, HarnessDigest: record.HarnessDigest, Compiler: record.Tool,
	}
	partitions := make([]semanticir.DomainPartitionEvidence, 0)
	partitionPaths := make(map[string]map[string]semanticir.LabelPathEvidence)
	operationScopes := make(map[string]semanticir.CompilerPredicate)
	for _, operation := range task.Operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		declarations, scopeFormula := proofOperationSMT(task, operation)
		scope := proofCompilerPredicate(record, declarations, scopeFormula, "scope-"+operation.ID)
		operationScopes[operation.ID] = scope
		record.OperationScopes = append(record.OperationScopes, semanticir.OperationScopeEvidence{
			OperationID: operation.ID, ScopePredicateDigest: scope.FormulaDigest, ScopePredicate: scope, Provenance: provenance,
		})
		for _, domainID := range operation.DomainIDs {
			var domain semanticir.Domain
			for _, candidate := range task.Domains {
				if candidate.ID == domainID {
					domain = candidate
					break
				}
			}
			partition := semanticir.DomainPartitionEvidence{OperationID: operation.ID, DomainID: domainID, ScopePredicateDigest: scope.FormulaDigest, ScopePredicate: scope, Totality: semanticir.ProofProved, Disjointness: semanticir.ProofProved, Provenance: provenance}
			paths := make(map[string]semanticir.LabelPathEvidence)
			var memberships []semanticir.CompilerPredicate
			for _, value := range domain.Values {
				membership := proofCompilerPredicate(record, declarations, proofSMTName(operation.ID, domainID, value.ID), "node-"+operation.ID+"-"+domainID+"-"+value.ID)
				witness := proofDomainWitness(domain, value)
				witnessDigest, err := semanticir.Digest(witness)
				if err != nil {
					panic(err)
				}
				claim := semanticir.NewProofClaim(semanticir.ClaimReachability, context, scope, []semanticir.CompilerPredicate{membership}, nil)
				replay := proofReplayableProof(claim, semanticir.SolverSAT, prover, environmentDigest)
				path := semanticir.LabelPathEvidence{
					ValueID: value.ID, PredicateDigest: membership.FormulaDigest, MembershipPredicate: membership, CompilerNodeIDs: append([]string(nil), membership.CompilerNodeIDs...),
					Reachability: semanticir.ProofProved, ReachabilityProofDigest: replay.QueryDigest, ReachabilityProof: replay,
					ConcreteWitness: &witness, WitnessDigest: witnessDigest, Provenance: provenance,
				}
				partition.Labels = append(partition.Labels, path)
				memberships = append(memberships, membership)
				paths[value.ID] = path
			}
			totalityClaim := semanticir.NewProofClaim(semanticir.ClaimTotality, context, scope, memberships, nil)
			partition.TotalityProof = proofReplayableProof(totalityClaim, semanticir.SolverUNSAT, prover, environmentDigest)
			partition.TotalityProofDigest = partition.TotalityProof.QueryDigest
			disjointnessClaim := semanticir.NewProofClaim(semanticir.ClaimDisjointness, context, scope, memberships, nil)
			partition.DisjointnessProof = proofReplayableProof(disjointnessClaim, semanticir.SolverUNSAT, prover, environmentDigest)
			partition.DisjointnessProofDigest = partition.DisjointnessProof.QueryDigest
			partitions = append(partitions, partition)
			partitionPaths[operation.ID+"/"+domainID] = paths
		}
	}
	for _, constraint := range task.Constraints {
		operation, found := findProofOperation(task.Operations, constraint.OperationID)
		if !found || len(operation.DomainIDs) == 0 {
			continue
		}
		var memberships []semanticir.CompilerPredicate
		for _, domainID := range operation.DomainIDs {
			path := partitionPaths[operation.ID+"/"+domainID][constraint.Conditions[domainID]]
			memberships = append(memberships, path.MembershipPredicate)
		}
		claim := semanticir.NewProofClaim(semanticir.ClaimExclusion, context, operationScopes[operation.ID], memberships, nil)
		replay := proofReplayableProof(claim, semanticir.SolverUNSAT, prover, environmentDigest)
		for partitionIndex := range partitions {
			if partitions[partitionIndex].OperationID == operation.ID && partitions[partitionIndex].DomainID == operation.DomainIDs[0] {
				partitions[partitionIndex].Exclusions = append(partitions[partitionIndex].Exclusions, semanticir.ConstraintPathEvidence{
					ConstraintID: constraint.ID, Result: semanticir.ProofProved, ProofDigest: replay.QueryDigest, Proof: replay, Provenance: provenance,
				})
				break
			}
		}
	}
	record.Partitions = partitions
	for _, behaviorCase := range task.CodeCases {
		operation, found := findProofOperation(task.Operations, behaviorCase.OperationID)
		if !found {
			continue
		}
		var memberships []semanticir.CompilerPredicate
		var categoryDigests []string
		for _, domainID := range operation.DomainIDs {
			path := partitionPaths[operation.ID+"/"+domainID][behaviorCase.Conditions[domainID]]
			memberships = append(memberships, path.MembershipPredicate)
			categoryDigests = append(categoryDigests, path.PredicateDigest)
		}
		sort.Strings(categoryDigests)
		var outcomes []semanticir.CompilerOutcomePredicate
		for _, outcomeID := range behaviorCase.OutcomeIDs {
			outcomes = append(outcomes, semanticir.CompilerOutcomePredicate{
				OutcomeID: outcomeID,
				Predicate: proofCompilerPredicate(record, operationScopes[operation.ID].Declarations, proofOutcomeSMTName(operation.ID, outcomeID), "outcome-"+operation.ID+"-"+outcomeID),
			})
		}
		claim := semanticir.NewProofClaim(semanticir.ClaimRealization, context, operationScopes[operation.ID], memberships, outcomes)
		replay := proofReplayableProof(claim, semanticir.SolverUNSAT, prover, environmentDigest)
		record.BehaviorProofs = append(record.BehaviorProofs, semanticir.BehaviorRealizationEvidence{
			BehaviorCaseID: behaviorCase.ID,
			Behavior:       semanticir.BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: assignmentCopy(behaviorCase.Conditions), Inputs: assignmentInputsCopy(behaviorCase.Inputs), Provenance: provenance},
			OutcomeIDs:     append([]string(nil), behaviorCase.OutcomeIDs...), CategoryPredicateDigests: categoryDigests,
			RealizationProof: replay, Provenance: provenance,
		})
	}
	for _, operation := range task.Operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		other := semanticir.OtherOutcome(operation.ID, provenance)
		var declared []semanticir.CompilerOutcomePredicate
		var memberships []semanticir.CompilerPredicate
		var complement semanticir.CompilerOutcomePredicate
		for _, outcomeID := range operation.OutcomeIDs {
			predicate := proofCompilerPredicate(record, operationScopes[operation.ID].Declarations, proofOutcomeSMTName(operation.ID, outcomeID), "closure-outcome-"+operation.ID+"-"+outcomeID)
			item := semanticir.CompilerOutcomePredicate{OutcomeID: outcomeID, Predicate: predicate}
			if outcomeID == other.ID {
				complement = item
			} else {
				declared = append(declared, item)
			}
			memberships = append(memberships, predicate)
		}
		totalClaim := semanticir.NewProofClaim(semanticir.ClaimTotality, context, operationScopes[operation.ID], memberships, nil)
		disjointClaim := semanticir.NewProofClaim(semanticir.ClaimDisjointness, context, operationScopes[operation.ID], memberships, nil)
		record.OutcomeClosures = append(record.OutcomeClosures, semanticir.OutcomeClosureEvidence{
			OperationID: operation.ID, BoundaryDigest: proofDigest("outcome-boundary-" + operation.ID), Declared: declared,
			Complements: []semanticir.OutcomeComplement{{ID: other.ID, Kind: semanticir.OutcomeComplementEffects, Description: "all other terminal, raise, nontermination, or ordered-effect traces", Predicate: complement}},
			Totality:    semanticir.ProofProved, TotalityProof: proofReplayableProof(totalClaim, semanticir.SolverUNSAT, prover, environmentDigest),
			Disjointness: semanticir.ProofProved, DisjointnessProof: proofReplayableProof(disjointClaim, semanticir.SolverUNSAT, prover, environmentDigest), Provenance: provenance,
		})
	}
	record.SemanticGraph = proofCompilerSemanticGraph(task, artifact, record)
	record.EmittedIRDigest = record.SemanticGraph.IRDigest
	record.TotalConstructs = len(record.SemanticGraph.Constructs)
	record.TranslatedConstructs = record.TotalConstructs
	graphDigest, err := semanticir.CompilerSemanticGraphDigest(record.SemanticGraph)
	if err != nil {
		panic(err)
	}
	record.FormulaDerivationDigest = graphDigest
	return []semanticir.CompilerEvidence{record}
}

func proofCompilerSemanticGraph(task *semanticir.Task, artifact semanticir.ArtifactRef, record semanticir.CompilerEvidence) *semanticir.CompilerSemanticGraph {
	provenance := proofProvenance(artifact)
	const constructID = "fixture-compiler-construct"
	graph := &semanticir.CompilerSemanticGraph{
		SourceDigest: artifact.Digest, WorkspaceTreeDigest: record.WorkspaceTreeDigest,
		Tool: record.Tool, IRKind: record.IRKind,
		Environment: []semanticir.EnvironmentVariable{}, EnvironmentDigest: proofReplayEnvironmentDigest(),
		Provenance: provenance,
	}

	// The fixture graph uses mathematical integers over the exact minimum and
	// maximum appearing in its finite points. Production frontends supply their
	// language/compiler numeric semantics instead.
	var integerValues []int64
	for _, operation := range task.Operations {
		for _, input := range operation.Inputs {
			for _, literal := range input.Universe {
				if literal.Type == semanticir.TypeInteger {
					integerValues = append(integerValues, literal.Integer)
				}
			}
		}
	}
	for _, behaviorCase := range task.CodeCases {
		for _, literal := range behaviorCase.Inputs {
			if literal.Type == semanticir.TypeInteger {
				integerValues = append(integerValues, literal.Integer)
			}
		}
	}
	for _, outcome := range task.Outcomes {
		if outcome.Value != nil && outcome.Value.Type == semanticir.TypeInteger {
			integerValues = append(integerValues, outcome.Value.Integer)
		}
		for _, effect := range outcome.Effects {
			if effect.Value != nil && effect.Value.Kind == semanticir.ExprLiteral && effect.Value.Literal != nil && effect.Value.Literal.Type == semanticir.TypeInteger {
				integerValues = append(integerValues, effect.Value.Literal.Integer)
			}
		}
	}
	if len(integerValues) != 0 {
		lower, upper := integerValues[0], integerValues[0]
		for _, value := range integerValues[1:] {
			if value < lower {
				lower = value
			}
			if value > upper {
				upper = value
			}
		}
		lowerLiteral := semanticir.Literal{Type: semanticir.TypeInteger, Integer: lower}
		upperLiteral := semanticir.Literal{Type: semanticir.TypeInteger, Integer: upper}
		graph.Numeric = []semanticir.CompilerNumericSemantics{{
			ID: "fixture-math-int", Kind: semanticir.CompilerNumericUnbounded, Signed: true,
			Overflow: semanticir.CompilerOverflowUnbounded, Range: semanticir.CompilerRangeBounded,
			LowerBound: &lowerLiteral, UpperBound: &upperLiteral,
		}}
	}

	outcomes := make(map[string]semanticir.ObservableOutcome, len(task.Outcomes))
	for _, outcome := range task.Outcomes {
		outcomes[outcome.ID] = outcome
	}
	operations := append([]semanticir.Operation(nil), task.Operations...)
	sort.Slice(operations, func(i, j int) bool { return operations[i].ID < operations[j].ID })
	for operationIndex, operation := range operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		var cases []semanticir.BehaviorCase
		for _, behaviorCase := range task.CodeCases {
			if behaviorCase.OperationID == operation.ID {
				cases = append(cases, behaviorCase)
			}
		}
		sort.Slice(cases, func(i, j int) bool {
			return semanticir.BehaviorCaseKey(cases[i]) < semanticir.BehaviorCaseKey(cases[j])
		})
		if len(cases) == 0 {
			panic("proof compiler graph operation has no concrete cases: " + operation.ID)
		}
		prefix := fmt.Sprintf("fixture-op-%d", operationIndex)
		inputNodeIDs := make(map[string]string, len(operation.Inputs))
		root := semanticir.CompilerOperationGraph{OperationID: operation.ID, Provenance: provenance}
		for inputIndex, input := range operation.Inputs {
			nodeID := fmt.Sprintf("%s-input-%d", prefix, inputIndex)
			node := semanticir.CompilerSemanticNode{
				ID: nodeID, Kind: semanticir.CompilerNodeInput, Type: input.Type, InputName: input.Name,
				CompilerNodeIDs: []string{constructID}, Provenance: provenance,
			}
			if input.Type == semanticir.TypeInteger {
				node.NumericID = "fixture-math-int"
			}
			graph.Nodes = append(graph.Nodes, node)
			inputNodeIDs[input.Name] = nodeID
			root.Inputs = append(root.Inputs, semanticir.CompilerInputNode{InputName: input.Name, NodeID: nodeID})
		}

		terminalBlocks := make([]string, len(cases))
		for caseIndex, behaviorCase := range cases {
			outcomeID := ""
			if len(behaviorCase.OutcomeIDs) != 0 {
				outcomeID = behaviorCase.OutcomeIDs[0]
			}
			outcome, exists := outcomes[outcomeID]
			if !exists {
				panic("proof compiler graph references an unknown outcome")
			}
			blockID := fmt.Sprintf("%s-terminal-%d", prefix, caseIndex)
			terminalBlocks[caseIndex] = blockID
			nodeIDs, terminalID := appendProofTerminalNodes(&graph.Nodes, prefix, caseIndex, outcome, constructID, provenance)
			graph.Blocks = append(graph.Blocks, semanticir.CompilerSemanticBlock{ID: blockID, NodeIDs: nodeIDs, CompilerNodeIDs: []string{constructID}, Provenance: provenance})
			root.TerminalNodeIDs = append(root.TerminalNodeIDs, terminalID)
		}

		if len(cases) == 1 {
			root.EntryBlockID = terminalBlocks[0]
			// Input nodes must be owned by the reachable entry block and dominate
			// terminal uses. They precede the terminal expression nodes.
			for inputIndex := len(operation.Inputs) - 1; inputIndex >= 0; inputIndex-- {
				inputID := root.Inputs[inputIndex].NodeID
				graph.Blocks[len(graph.Blocks)-1].NodeIDs = append([]string{inputID}, graph.Blocks[len(graph.Blocks)-1].NodeIDs...)
			}
		} else {
			branchBlocks := make([]string, len(cases)-1)
			for caseIndex := range branchBlocks {
				branchBlocks[caseIndex] = fmt.Sprintf("%s-branch-%d", prefix, caseIndex)
			}
			root.EntryBlockID = branchBlocks[0]
			for caseIndex, blockID := range branchBlocks {
				var nodeIDs []string
				if caseIndex == 0 {
					for _, input := range root.Inputs {
						nodeIDs = append(nodeIDs, input.NodeID)
					}
				}
				guardID := appendProofCaseGuard(&graph.Nodes, &nodeIDs, prefix, caseIndex, operation, cases[caseIndex], inputNodeIDs, constructID, provenance)
				graph.Blocks = append(graph.Blocks, semanticir.CompilerSemanticBlock{ID: blockID, NodeIDs: nodeIDs, CompilerNodeIDs: []string{constructID}, Provenance: provenance})
				falseTarget := terminalBlocks[len(cases)-1]
				if caseIndex+1 < len(branchBlocks) {
					falseTarget = branchBlocks[caseIndex+1]
				}
				graph.Edges = append(graph.Edges,
					semanticir.CompilerControlEdge{ID: blockID + "-true", FromBlockID: blockID, ToBlockID: terminalBlocks[caseIndex], GuardNodeID: guardID, GuardValue: true, CompilerNodeIDs: []string{constructID}, Provenance: provenance},
					semanticir.CompilerControlEdge{ID: blockID + "-false", FromBlockID: blockID, ToBlockID: falseTarget, GuardNodeID: guardID, GuardValue: false, CompilerNodeIDs: []string{constructID}, Provenance: provenance},
				)
			}
		}
		graph.Operations = append(graph.Operations, root)
	}

	semanticNodeIDs := make([]string, 0, len(graph.Nodes))
	blockIDs := make([]string, 0, len(graph.Blocks))
	edgeIDs := make([]string, 0, len(graph.Edges))
	for _, node := range graph.Nodes {
		semanticNodeIDs = append(semanticNodeIDs, node.ID)
	}
	for _, block := range graph.Blocks {
		blockIDs = append(blockIDs, block.ID)
	}
	for _, edge := range graph.Edges {
		edgeIDs = append(edgeIDs, edge.ID)
	}
	graph.Constructs = []semanticir.CompilerConstructBinding{{
		ID: constructID, Kind: semanticir.CompilerConstructControl, Opcode: "branch",
		SemanticNodeIDs: semanticNodeIDs, BlockIDs: blockIDs, EdgeIDs: edgeIDs, Provenance: provenance,
	}}
	graph.IR = []byte("fixture-compiler-ir\x00" + artifact.Digest)
	graph.IRDigest = semanticir.DigestBytes(graph.IR)
	graph.DerivationSteps = []semanticir.ProbeStep{proofRawProbeStep("derive-compiler-ir", record.Tool, graph.IR, provenance)}
	decoderOutput, err := semanticir.CanonicalCompilerDecoderOutput(graph)
	if err != nil {
		panic(err)
	}
	graph.DecoderOutput = decoderOutput
	graph.DecoderOutputDigest = semanticir.DigestBytes(decoderOutput)
	graph.DecoderSteps = []semanticir.ProbeStep{proofRawProbeStep("decode-compiler-ir", record.Tool, decoderOutput, provenance)}
	return graph
}

func appendProofCaseGuard(nodes *[]semanticir.CompilerSemanticNode, blockNodeIDs *[]string, prefix string, caseIndex int, operation semanticir.Operation, behaviorCase semanticir.BehaviorCase, inputNodeIDs map[string]string, constructID string, provenance semanticir.Provenance) string {
	var comparisons []string
	for inputIndex, input := range operation.Inputs {
		literal, exists := behaviorCase.Inputs[input.Name]
		if !exists || literal.Type != input.Type {
			panic("proof compiler graph case lacks a typed concrete input")
		}
		constantID := fmt.Sprintf("%s-case-%d-constant-%d", prefix, caseIndex, inputIndex)
		constant := semanticir.CompilerSemanticNode{ID: constantID, Kind: semanticir.CompilerNodeConstant, Type: literal.Type, Literal: &literal, CompilerNodeIDs: []string{constructID}, Provenance: provenance}
		if literal.Type == semanticir.TypeInteger {
			constant.NumericID = "fixture-math-int"
		}
		comparisonID := fmt.Sprintf("%s-case-%d-eq-%d", prefix, caseIndex, inputIndex)
		comparison := semanticir.CompilerSemanticNode{ID: comparisonID, Kind: semanticir.CompilerNodeEQ, Type: semanticir.TypeBool, Operands: []string{inputNodeIDs[input.Name], constantID}, CompilerNodeIDs: []string{constructID}, Provenance: provenance}
		*nodes = append(*nodes, constant, comparison)
		*blockNodeIDs = append(*blockNodeIDs, constantID, comparisonID)
		comparisons = append(comparisons, comparisonID)
	}
	if len(comparisons) == 0 {
		panic("proof compiler graph has multiple cases for a zero-input operation")
	}
	guardID := comparisons[0]
	for index := 1; index < len(comparisons); index++ {
		andID := fmt.Sprintf("%s-case-%d-and-%d", prefix, caseIndex, index)
		*nodes = append(*nodes, semanticir.CompilerSemanticNode{ID: andID, Kind: semanticir.CompilerNodeAnd, Type: semanticir.TypeBool, Operands: []string{guardID, comparisons[index]}, CompilerNodeIDs: []string{constructID}, Provenance: provenance})
		*blockNodeIDs = append(*blockNodeIDs, andID)
		guardID = andID
	}
	return guardID
}

func appendProofTerminalNodes(nodes *[]semanticir.CompilerSemanticNode, prefix string, caseIndex int, outcome semanticir.ObservableOutcome, constructID string, provenance semanticir.Provenance) ([]string, string) {
	var nodeIDs []string
	var effectIDs []string
	for effectIndex, effect := range outcome.Effects {
		effectID := fmt.Sprintf("%s-terminal-%d-effect-%d", prefix, caseIndex, effectIndex)
		effectNode := semanticir.CompilerSemanticNode{ID: effectID, Kind: semanticir.CompilerNodeEffect, Type: semanticir.TypeUnit, EffectKind: effect.Kind, EffectTarget: effect.Target, CompilerNodeIDs: []string{constructID}, Provenance: provenance}
		if effect.Value != nil {
			if effect.Value.Kind != semanticir.ExprLiteral || effect.Value.Literal == nil {
				panic("proof compiler graph fixture supports only literal effect values")
			}
			literal := *effect.Value.Literal
			valueID := effectID + "-value"
			valueNode := semanticir.CompilerSemanticNode{ID: valueID, Kind: semanticir.CompilerNodeConstant, Type: literal.Type, Literal: &literal, CompilerNodeIDs: []string{constructID}, Provenance: provenance}
			if literal.Type == semanticir.TypeInteger {
				valueNode.NumericID = "fixture-math-int"
			}
			*nodes = append(*nodes, valueNode)
			nodeIDs = append(nodeIDs, valueID)
			effectNode.Operands = []string{valueID}
		}
		*nodes = append(*nodes, effectNode)
		nodeIDs = append(nodeIDs, effectID)
		effectIDs = append(effectIDs, effectID)
	}
	terminalID := fmt.Sprintf("%s-terminal-%d-result", prefix, caseIndex)
	terminal := semanticir.CompilerSemanticNode{ID: terminalID, EffectNodeIDs: effectIDs, CompilerNodeIDs: []string{constructID}, Provenance: provenance}
	switch outcome.Kind {
	case semanticir.OutcomeReturn:
		if outcome.Value == nil {
			panic("proof compiler graph return outcome has no value")
		}
		literal := *outcome.Value
		valueID := terminalID + "-value"
		valueNode := semanticir.CompilerSemanticNode{ID: valueID, Kind: semanticir.CompilerNodeConstant, Type: literal.Type, Literal: &literal, CompilerNodeIDs: []string{constructID}, Provenance: provenance}
		if literal.Type == semanticir.TypeInteger {
			valueNode.NumericID = "fixture-math-int"
			terminal.NumericID = "fixture-math-int"
		}
		*nodes = append(*nodes, valueNode)
		nodeIDs = append(nodeIDs, valueID)
		terminal.Kind, terminal.Type, terminal.Operands = semanticir.CompilerNodeReturn, literal.Type, []string{valueID}
	case semanticir.OutcomeRaise:
		terminal.Kind, terminal.Type = semanticir.CompilerNodeRaise, semanticir.TypeUnit
		terminal.ExceptionType, terminal.Message = outcome.ExceptionType, outcome.Message
	case semanticir.OutcomeSuccess:
		terminal.Kind, terminal.Type = semanticir.CompilerNodeSuccess, semanticir.TypeUnit
	case semanticir.OutcomeOther:
		literal := semanticir.Literal{Type: semanticir.TypeString, String: "__ray_proof_fixture_other__"}
		valueID := terminalID + "-other-value"
		*nodes = append(*nodes, semanticir.CompilerSemanticNode{ID: valueID, Kind: semanticir.CompilerNodeConstant, Type: literal.Type, Literal: &literal, CompilerNodeIDs: []string{constructID}, Provenance: provenance})
		nodeIDs = append(nodeIDs, valueID)
		terminal.Kind, terminal.Type, terminal.Operands = semanticir.CompilerNodeReturn, literal.Type, []string{valueID}
	default:
		panic("proof compiler graph fixture has unsupported outcome kind")
	}
	*nodes = append(*nodes, terminal)
	nodeIDs = append(nodeIDs, terminalID)
	return nodeIDs, terminalID
}

func proofRawProbeStep(id string, tool semanticir.ToolRef, output []byte, provenance semanticir.Provenance) semanticir.ProbeStep {
	return semanticir.ProbeStep{
		ID: id, Kind: semanticir.ProbeStepRun, Tool: tool,
		Argv:        []string{"-proof-replay-raw=" + base64.RawURLEncoding.EncodeToString(output)},
		StdinDigest: semanticir.DigestBytes(nil), WorkingDirectory: proofWorkspaceRoot(),
		Environment: []semanticir.EnvironmentVariable{}, EnvironmentDigest: proofReplayEnvironmentDigest(),
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000, ExpectedExitCode: 0,
		ExpectedStdoutDigest: semanticir.DigestBytes(output), ExpectedStderrDigest: semanticir.DigestBytes(nil), ExpectedSignalDigest: semanticir.DigestBytes(nil),
		SignalExtractor: semanticir.ProbeSignalExtractor{Kind: semanticir.ProbeSignalNone}, Provenance: provenance,
	}
}

func compilerContext(evidence semanticir.CompilerEvidence) semanticir.CompilerProofContext {
	return semanticir.CompilerProofContext{
		SourceDigest: evidence.SourceDigest, WorkspaceTreeDigest: evidence.WorkspaceTreeDigest,
		EmittedIRDigest: evidence.EmittedIRDigest, HarnessDigest: evidence.HarnessDigest, Compiler: evidence.Tool,
	}
}

func proofCompilerPredicate(evidence semanticir.CompilerEvidence, declarations []byte, formula string, nodeIDs ...string) semanticir.CompilerPredicate {
	return semanticir.CompilerPredicate{
		Logic: semanticir.ProofLogicSMTLIB2, Declarations: append([]byte(nil), declarations...), DeclarationsDigest: semanticir.DigestBytes(declarations),
		Formula: []byte(formula), FormulaDigest: semanticir.DigestBytes([]byte(formula)), Tool: evidence.Tool, IRDigest: evidence.EmittedIRDigest,
		CompilerNodeIDs: append([]string(nil), nodeIDs...),
	}
}

func proofReplayableProof(claim semanticir.ProofClaim, result semanticir.SolverResult, prover semanticir.ToolRef, environmentDigest string) semanticir.ReplayableProof {
	query, err := semanticir.CanonicalProofQuery(claim)
	if err != nil {
		panic(err)
	}
	output := []byte(string(result) + "\n")
	return semanticir.ReplayableProof{
		Claim: claim, Logic: semanticir.ProofLogicSMTLIB2, Query: query, QueryDigest: semanticir.DigestBytes(query),
		Prover: prover, Argv: []string{"-in", "-smt2"}, WorkingDirectory: proofWorkspaceRoot(),
		Environment: []semanticir.EnvironmentVariable{}, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000,
		SolverOutput: output, SolverOutputDigest: semanticir.DigestBytes(output), Result: result, SubjectDigests: semanticir.ProofClaimSubjectDigests(claim),
	}
}

func proofOperationSMT(task *semanticir.Task, operation semanticir.Operation) ([]byte, string) {
	var declarations strings.Builder
	var scopeParts []string
	for _, domainID := range operation.DomainIDs {
		domain := findProofDomain(task.Domains, domainID)
		var variables []string
		for _, value := range domain.Values {
			variable := proofSMTName(operation.ID, domainID, value.ID)
			variables = append(variables, variable)
			declarations.WriteString("(declare-const ")
			declarations.WriteString(variable)
			declarations.WriteString(" Bool)\n")
		}
		scopeParts = append(scopeParts, "(or "+strings.Join(variables, " ")+")")
		for left := range variables {
			for right := left + 1; right < len(variables); right++ {
				scopeParts = append(scopeParts, "(not (and "+variables[left]+" "+variables[right]+"))")
			}
		}
	}
	var outcomeVariables []string
	for _, outcomeID := range operation.OutcomeIDs {
		variable := proofOutcomeSMTName(operation.ID, outcomeID)
		outcomeVariables = append(outcomeVariables, variable)
		declarations.WriteString("(declare-const ")
		declarations.WriteString(variable)
		declarations.WriteString(" Bool)\n")
	}
	if len(outcomeVariables) != 0 {
		scopeParts = append(scopeParts, "(or "+strings.Join(outcomeVariables, " ")+")")
		for left := range outcomeVariables {
			for right := left + 1; right < len(outcomeVariables); right++ {
				scopeParts = append(scopeParts, "(not (and "+outcomeVariables[left]+" "+outcomeVariables[right]+"))")
			}
		}
	}
	for _, constraint := range task.Constraints {
		if constraint.OperationID != operation.ID {
			continue
		}
		var categories []string
		for _, domainID := range operation.DomainIDs {
			categories = append(categories, proofSMTName(operation.ID, domainID, constraint.Conditions[domainID]))
		}
		scopeParts = append(scopeParts, "(not "+proofSMTAnd(categories)+")")
	}
	for _, behaviorCase := range task.CodeCases {
		if behaviorCase.OperationID != operation.ID || len(behaviorCase.OutcomeIDs) != 1 {
			continue
		}
		var categories []string
		for _, domainID := range operation.DomainIDs {
			categories = append(categories, proofSMTName(operation.ID, domainID, behaviorCase.Conditions[domainID]))
		}
		scopeParts = append(scopeParts, "(=> "+proofSMTAnd(categories)+" "+proofOutcomeSMTName(operation.ID, behaviorCase.OutcomeIDs[0])+")")
	}
	return []byte(strings.TrimSpace(declarations.String())), proofSMTAnd(scopeParts)
}

func proofSMTAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return "true"
	case 1:
		return parts[0]
	default:
		return "(and " + strings.Join(parts, " ") + ")"
	}
}

func proofOutcomeSMTName(operationID, outcomeID string) string {
	return proofSMTName("outcome", operationID, outcomeID)
}

func proofSMTName(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "ray_v_" + hex.EncodeToString(digest[:8])
}

func proofDomainWitness(domain semanticir.Domain, value semanticir.DomainValue) semanticir.Literal {
	if value.Value != nil {
		return *value.Value
	}
	return semanticir.Literal{Type: domain.Type, String: value.ID}
}

func findProofDomain(domains []semanticir.Domain, id string) semanticir.Domain {
	for _, domain := range domains {
		if domain.ID == id {
			return domain
		}
	}
	return semanticir.Domain{}
}

func findProofOperation(operations []semanticir.Operation, id string) (semanticir.Operation, bool) {
	for _, operation := range operations {
		if operation.ID == id {
			return operation, true
		}
	}
	return semanticir.Operation{}, false
}

func proofModelOperation(operation semanticir.Operation, artifact semanticir.ArtifactRef) semanticir.Operation {
	result := operation
	result.Provenance = proofProvenance(artifact)
	result.Inputs = append([]semanticir.Variable(nil), operation.Inputs...)
	for i := range result.Inputs {
		result.Inputs[i].Provenance = proofProvenance(artifact)
	}
	return result
}

func truePredicate() semanticir.TestPredicate {
	artifact := proofArtifact("tests", semanticir.ArtifactTests)
	return semanticir.TestPredicate{Kind: semanticir.PredicateTrue, Provenance: proofProvenance(artifact)}
}

func falsePredicate() semanticir.TestPredicate {
	artifact := proofArtifact("tests", semanticir.ArtifactTests)
	return semanticir.TestPredicate{Kind: semanticir.PredicateFalse, Provenance: proofProvenance(artifact)}
}

func andPredicate(children ...semanticir.TestPredicate) semanticir.TestPredicate {
	artifact := proofArtifact("tests", semanticir.ArtifactTests)
	return semanticir.TestPredicate{Kind: semanticir.PredicateAnd, Children: children, Provenance: proofProvenance(artifact)}
}

func outcomeIn(id, operation string, conditions semanticir.Assignment, outcomes ...string) semanticir.TestPredicate {
	artifact := proofArtifact("tests", semanticir.ArtifactTests)
	provenance := proofProvenance(artifact)
	return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &semanticir.Observation{
		Kind: semanticir.ObserveOutcome, Behavior: semanticir.BehaviorRef{OperationID: operation, Conditions: conditions, Inputs: proofBehaviorInputs(conditions), Provenance: provenance}, OutcomeIDs: outcomes, Provenance: provenance,
	}, Provenance: provenance}
}

func outcomeEqual(id, leftOperation string, left semanticir.Assignment, rightOperation string, right semanticir.Assignment) semanticir.TestPredicate {
	artifact := proofArtifact("tests", semanticir.ArtifactTests)
	provenance := proofProvenance(artifact)
	return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeEqual,
		Left:  &semanticir.BehaviorRef{OperationID: leftOperation, Conditions: left, Inputs: proofBehaviorInputs(left), Provenance: provenance},
		Right: &semanticir.BehaviorRef{OperationID: rightOperation, Conditions: right, Inputs: proofBehaviorInputs(right), Provenance: provenance}, Provenance: provenance,
	}
}

func hasEffect(id, operation string, conditions semanticir.Assignment, kind semanticir.EffectKind, target string) semanticir.TestPredicate {
	artifact := proofArtifact("tests", semanticir.ArtifactTests)
	provenance := proofProvenance(artifact)
	return semanticir.TestPredicate{Kind: semanticir.PredicateHasEffect, Observe: &semanticir.Observation{
		Kind: semanticir.ObserveEffect, Behavior: semanticir.BehaviorRef{OperationID: operation, Conditions: conditions, Inputs: proofBehaviorInputs(conditions), Provenance: provenance}, EffectKind: kind, EffectTarget: target, Provenance: provenance,
	}, Provenance: provenance}
}

func hasEffectValue(id, operation string, conditions semanticir.Assignment, value string) semanticir.TestPredicate {
	artifact := proofArtifact("tests", semanticir.ArtifactTests)
	provenance := proofProvenance(artifact)
	expected := literalStringExpression(artifact, value)
	return semanticir.TestPredicate{Kind: semanticir.PredicateHasEffect, Observe: &semanticir.Observation{
		Kind: semanticir.ObserveEffect, Behavior: semanticir.BehaviorRef{OperationID: operation, Conditions: conditions, Inputs: proofBehaviorInputs(conditions), Provenance: provenance},
		EffectKind: semanticir.EffectWrite, EffectTarget: "audit", EffectValue: &expected, Provenance: provenance,
	}, Provenance: provenance}
}

func literalStringExpression(artifact semanticir.ArtifactRef, value string) semanticir.Expression {
	return semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeString, Literal: &semanticir.Literal{Type: semanticir.TypeString, String: value}, Provenance: proofProvenance(artifact)}
}

func proofEqualityMembership(artifact semanticir.ArtifactRef, name string, literal semanticir.Literal) semanticir.Expression {
	provenance := proofProvenance(artifact)
	return semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: semanticir.OpEQ, Operands: []semanticir.Expression{
		{Kind: semanticir.ExprVariable, Type: literal.Type, Name: name, Provenance: provenance},
		{Kind: semanticir.ExprLiteral, Type: literal.Type, Literal: &literal, Provenance: provenance},
	}, Provenance: provenance}
}

func proofBehaviorInputs(conditions semanticir.Assignment) map[string]semanticir.Literal {
	result := make(map[string]semanticir.Literal, len(conditions))
	for name, value := range conditions {
		integer, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			integer = map[string]int64{"p": 0, "q": 1, "r": 2}[value]
		}
		result[name] = semanticir.Literal{Type: semanticir.TypeInteger, Integer: integer}
	}
	return result
}

func proofWitnessContainsOutcome(witness *semanticir.Counterexample, outcomeID string) bool {
	if witness == nil {
		return false
	}
	for _, choice := range witness.Choices {
		if choice.OutcomeID == outcomeID {
			return true
		}
	}
	return false
}

func assignmentInputsCopy(inputs map[string]semanticir.Literal) map[string]semanticir.Literal {
	result := make(map[string]semanticir.Literal, len(inputs))
	for name, literal := range inputs {
		result[name] = literal
	}
	return result
}

func proofCommands(artifact semanticir.ArtifactRef) []semanticir.WorkspaceCommand {
	commands := make([]semanticir.WorkspaceCommand, 3)
	states := []semanticir.WorkspaceState{semanticir.WorkspaceBaseOldTests, semanticir.WorkspaceBaseNewTests, semanticir.WorkspaceSolutionNewTests}
	for i, workspace := range []string{"base-old", "base-new", "solution-new"} {
		commands[i] = semanticir.WorkspaceCommand{
			ID: workspace, WorkspaceID: workspace, State: states[i], TreeDigest: proofWorkspaceDigest(), Command: "./tests/test.sh", WorkingDirectory: proofWorkspaceRoot(), EnvironmentDigest: proofReplayEnvironmentDigest(), TimeoutMillis: 1000,
			Environment: []semanticir.EnvironmentVariable{}, ClearEnvironment: true, KillProcessGroup: true,
			ExpectedPass: true, ObservedPass: true, ExitCode: 0, StdoutDigest: proofDigest(workspace + "-stdout"), StderrDigest: proofDigest(workspace + "-stderr"), SignalValueDigest: proofDigest(workspace + "-signal"),
			Tools: []semanticir.ToolRef{proofTool("runner")}, Provenance: proofProvenance(artifact),
			PassSignal: semanticir.PassSignal{Kind: semanticir.PassSignalExitCode, Expected: "0", Provenance: proofProvenance(artifact)},
		}
	}
	return commands
}

var (
	proofExecutableOnce   sync.Once
	proofExecutablePath   string
	proofExecutableDigest string
	proofWorkspaceOnce    sync.Once
	proofWorkspacePath    string
	proofWorkspaceSHA256  string
	proofZ3Once           sync.Once
	proofZ3Tool           semanticir.ToolRef
)

func proofTool(name string) semanticir.ToolRef {
	if name == "finite-prover" {
		proofZ3Once.Do(func() {
			path, err := exec.LookPath("z3")
			if err != nil {
				panic("proof tests require z3: " + err.Error())
			}
			body, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			version, err := exec.Command(path, "-version").Output()
			if err != nil {
				panic(err)
			}
			proofZ3Tool = semanticir.ToolRef{Name: "z3", Path: path, Digest: semanticir.DigestBytes(body), Version: strings.TrimSpace(string(version))}
		})
		return proofZ3Tool
	}
	proofExecutableOnce.Do(func() {
		var err error
		proofExecutablePath, err = os.Executable()
		if err != nil {
			panic(err)
		}
		file, err := os.Open(proofExecutablePath)
		if err != nil {
			panic(err)
		}
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			_ = file.Close()
			panic(err)
		}
		if err := file.Close(); err != nil {
			panic(err)
		}
		proofExecutableDigest = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	})
	return semanticir.ToolRef{Name: name, Path: proofExecutablePath, Digest: proofExecutableDigest, Version: "proof-replay-helper-v1"}
}

func proofReplayEnvironmentDigest() string {
	digest, err := semanticir.Digest([]semanticir.EnvironmentVariable{})
	if err != nil {
		panic(err)
	}
	return digest
}

func proofCoverage(artifact semanticir.ArtifactRef) semanticir.TranslationCoverage {
	return semanticir.TranslationCoverage{Status: semanticir.TranslationComplete, TotalConstructs: 1, TranslatedConstructs: 1, Provenance: proofProvenance(artifact)}
}

func environmentConfigurationForTest(environment *semanticir.EnvironmentModel) semanticir.ArtifactRef {
	if environment.Configuration != (semanticir.ArtifactRef{}) {
		return environment.Configuration
	}
	return environment.Artifact
}

func proofArtifact(id string, kind semanticir.ArtifactKind) semanticir.ArtifactRef {
	path := proofWorkspaceRoot() + "/" + id
	return semanticir.ArtifactRef{ID: id, Kind: kind, Path: path, Digest: proofDigest(id)}
}

func proofWorkspaceRoot() string {
	proofWorkspaceOnce.Do(func() {
		base := "/Volumes/Hak_SSD"
		if _, statErr := os.Stat(base); statErr != nil {
			base = os.TempDir()
		}
		root, err := os.MkdirTemp(base, "ray-proof-fixture-")
		if err != nil {
			panic(err)
		}
		for _, id := range []string{"instruction", "spec", "code", "tests", "configuration", "environment"} {
			if err := os.WriteFile(root+"/"+id, []byte(id), 0o600); err != nil {
				panic(err)
			}
		}
		proofWorkspacePath = root
		proofWorkspaceSHA256, err = executor.WorkspaceDigest(root)
		if err != nil {
			panic(err)
		}
	})
	return proofWorkspacePath
}

func proofWorkspaceDigest() string {
	_ = proofWorkspaceRoot()
	return proofWorkspaceSHA256
}

func proofDigest(seed string) string {
	digest := sha256.Sum256([]byte(seed))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func proofProvenance(artifact semanticir.ArtifactRef) semanticir.Provenance {
	return semanticir.NewProvenance(artifact, semanticir.SourceLocation{Path: artifact.Path, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 0}, semanticir.TranslationTranslated)
}

func assignment(domain, value string) semanticir.Assignment {
	return semanticir.Assignment{domain: value}
}

func subtractOutcomes(all, included []string) []string {
	set := make(map[string]bool)
	for _, value := range included {
		set[value] = true
	}
	var result []string
	for _, value := range all {
		if !set[value] {
			result = append(result, value)
		}
	}
	return result
}

func assertProofVerdict(t *testing.T, result proof.Result, want proof.Verdict) {
	t.Helper()
	if result.Verdict != want {
		t.Fatalf("verdict = %s, want %s; blockers=%+v", result.Verdict, want, result.Blockers)
	}
}

func assertProofEngineBlocked(t *testing.T, result proof.Result, code string) {
	t.Helper()
	if result.Verdict != proof.VerdictProofBlocked || result.Reference.Verdict != proof.VerdictProofBlocked || result.FalsePositive.Verdict != proof.VerdictProofBlocked || result.Fairness.Verdict != proof.VerdictProofBlocked || result.ReferenceAcceptance.Verdict != proof.VerdictProofBlocked {
		t.Fatalf("result = %+v, want all obligations PROOF BLOCKED", result)
	}
	for _, blocker := range result.Blockers {
		if blocker.Code == code {
			return
		}
	}
	t.Fatalf("blockers = %+v, want code %q", result.Blockers, code)
}
