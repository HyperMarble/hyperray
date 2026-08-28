package proof

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// Verify validates the complete finite universe and proves all three required
// set inclusions. It never treats sampling or execution of individual mutants
// as evidence that a counterexample does not exist.
func Verify(ctx context.Context, task *semanticir.Task) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	model, blockers := validate(ctx, task)
	if len(blockers) != 0 {
		return blockedResult(blockers, Transcript{})
	}

	transcript := Transcript{
		Method:                   enumerationMethod,
		Complete:                 true,
		DomainAssignments:        model.totalAssignments,
		ExcludedAssignments:      model.excluded,
		ReachableAssignments:     model.reachableCount,
		ReachableCases:           uint64(len(model.cases)),
		OutcomeUniverse:          uint64(len(model.outcomeIDs)),
		SpecIRDigest:             model.specIRDigest,
		ReferenceIRDigest:        model.referenceIRDigest,
		TestIRDigest:             model.testIRDigest,
		EnvironmentIRDigest:      model.environmentIRDigest,
		CompilerEvidence:         append([]semanticir.CompilerEvidence(nil), model.compilerEvidence...),
		CompilerEvidenceSHA256:   model.compilerEvidenceSHA256,
		DerivationReplays:        append([]DerivationReplayBinding(nil), model.derivationReplays...),
		DerivationReplaysSHA256:  model.derivationReplaysSHA256,
		ScopeClosures:            append([]semanticir.ScopeClosureEvidence(nil), model.scopeClosures...),
		ScopeClosuresSHA256:      model.scopeClosuresSHA256,
		ExhaustiveEvidence:       append([]semanticir.ExhaustiveExecutionEvidence(nil), model.exhaustiveEvidence...),
		ExhaustiveEvidenceSHA256: model.exhaustiveEvidenceSHA256,
		TestSuite:                cloneTestSuite(model.testSuite),
		TestSuiteSHA256:          model.testSuiteSHA256,
	}
	if requiresSolver(model) {
		reference, falsePositive, fairness, referenceAcceptance, solverTranscript, err := proveWithZ3(ctx, model, task.Environment)
		transcript.Method = "z3-qf-lia"
		transcript.Solver = solverTranscript
		if err != nil {
			return blockedResult([]Blocker{{Code: "solver-blocked", Message: err.Error()}}, transcript)
		}
		return completedResult(reference, falsePositive, fairness, referenceAcceptance, transcript)
	}

	reference, err := proveReference(ctx, model)
	if err != nil {
		return blockedResult([]Blocker{{Code: "proof-enumeration-blocked", Message: err.Error()}}, transcript)
	}
	falsePositive, err := proveFalsePositive(ctx, model)
	if err != nil {
		return blockedResult([]Blocker{{Code: "proof-enumeration-blocked", Message: err.Error()}}, transcript)
	}
	fairness, err := proveFairness(ctx, model)
	if err != nil {
		return blockedResult([]Blocker{{Code: "proof-enumeration-blocked", Message: err.Error()}}, transcript)
	}
	referenceAcceptance, err := proveReferenceAcceptance(model)
	if err != nil {
		return blockedResult([]Blocker{{Code: "proof-reference-acceptance-blocked", Message: err.Error()}}, transcript)
	}

	return completedResult(reference, falsePositive, fairness, referenceAcceptance, transcript)
}

func completedResult(reference, falsePositive, fairness, referenceAcceptance ObligationResult, transcript Transcript) Result {
	result := Result{
		Verdict:             VerdictVerified,
		Reference:           reference,
		FalsePositive:       falsePositive,
		Fairness:            fairness,
		ReferenceAcceptance: referenceAcceptance,
		Transcript:          transcript,
	}
	for _, obligation := range []*ObligationResult{&result.Reference, &result.FalsePositive, &result.Fairness, &result.ReferenceAcceptance} {
		if obligation.Witness != nil {
			result.Counterexamples = append(result.Counterexamples, *obligation.Witness)
		}
		if obligation.Verdict != VerdictVerified {
			result.Verdict = VerdictNotVerified
		}
	}
	return result
}

func proveReferenceAcceptance(model *finiteModel) (ObligationResult, error) {
	result := newObligation(semanticir.ObligationReferenceAcceptance, model)
	vector, err := referenceBehaviorVector(model)
	if err != nil {
		return result, err
	}
	result.OutcomeChecks = uint64(len(model.cases))
	passes, failedTest, err := evaluateSuite(model, vector)
	if err != nil {
		return result, err
	}
	if passes {
		return result, nil
	}
	highlight := fairnessHighlight(model, failedTest)
	requirement := &highlight.requirements[0]
	provenance := requirement.Provenance
	if failedTest != nil {
		provenance = failedTest.Provenance
	}
	result.Verdict = VerdictNotVerified
	result.Witness = makeCounterexample(semanticir.ObligationReferenceAcceptance, model, vector, highlight, requirement, false, provenance)
	return result, nil
}

func referenceBehaviorVector(model *finiteModel) (behaviorVector, error) {
	vector := make(behaviorVector, len(model.cases))
	for _, finiteCase := range model.cases {
		outcomes := sortedUnique(finiteCase.code.OutcomeIDs)
		if len(outcomes) != 1 {
			return nil, fmt.Errorf("reference point %q has %d outcomes; want exactly one", semanticir.BehaviorCaseKey(finiteCase.code), len(outcomes))
		}
		vector[finiteCaseKey(model, finiteCase)] = outcomes[0]
	}
	return vector, nil
}

func requiresSolver(model *finiteModel) bool {
	// Each inclusion performs a complete pass over its finite behavior vectors.
	// Keep the in-process enumerator at a deliberately small,
	// auditable bound; larger finite products go to the frozen exact solver.
	const maxEnumeratedVectors = uint64(1 << 12)
	product := uint64(1)
	for _, finiteCase := range model.cases {
		count := uint64(len(model.operationOutcomes[finiteCase.operation]))
		if count == 0 || product > maxEnumeratedVectors/count {
			return true
		}
		product *= count
	}
	return product > maxEnumeratedVectors
}

func proveReference(ctx context.Context, model *finiteModel) (ObligationResult, error) {
	result := newObligation(semanticir.ObligationReferenceCorrectness, model)
	domains := make([][]string, len(model.cases))
	for i := range model.cases {
		domains[i] = sortedUnique(model.cases[i].code.OutcomeIDs)
	}
	err := enumerateBehaviorVectors(ctx, model, domains, func(vector behaviorVector) error {
		if result.OutcomeChecks > math.MaxUint64-uint64(len(model.cases)) {
			return fmt.Errorf("finite outcome-check count exceeds proof accounting capacity")
		}
		result.OutcomeChecks += uint64(len(model.cases))
		_, violation := specSatisfied(model, vector)
		if violation != nil && result.Witness == nil {
			testPasses, _, err := evaluateSuite(model, vector)
			if err != nil {
				return err
			}
			result.Witness = makeCounterexample(semanticir.ObligationReferenceCorrectness, model, vector, violation.finiteCase, violation.requirement, testPasses, violation.finiteCase.code.Provenance)
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	finishObligation(&result)
	return result, nil
}

func proveFalsePositive(ctx context.Context, model *finiteModel) (ObligationResult, error) {
	result := newObligation(semanticir.ObligationTestsSound, model)
	domains := uniformDomains(model)
	err := enumerateBehaviorVectors(ctx, model, domains, func(vector behaviorVector) error {
		if result.OutcomeChecks > math.MaxUint64-uint64(len(model.cases)) {
			return fmt.Errorf("finite outcome-check count exceeds proof accounting capacity")
		}
		result.OutcomeChecks += uint64(len(model.cases))
		testsPass, _, err := evaluateSuite(model, vector)
		if err != nil {
			return err
		}
		if !testsPass {
			return nil
		}
		_, violation := specSatisfied(model, vector)
		if violation != nil && result.Witness == nil {
			result.Witness = makeCounterexample(semanticir.ObligationTestsSound, model, vector, violation.finiteCase, violation.requirement, true, violation.requirement.Provenance)
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	finishObligation(&result)
	return result, nil
}

func proveFairness(ctx context.Context, model *finiteModel) (ObligationResult, error) {
	result := newObligation(semanticir.ObligationTestsComplete, model)
	domains := uniformDomains(model)
	err := enumerateBehaviorVectors(ctx, model, domains, func(vector behaviorVector) error {
		if result.OutcomeChecks > math.MaxUint64-uint64(len(model.cases)) {
			return fmt.Errorf("finite outcome-check count exceeds proof accounting capacity")
		}
		result.OutcomeChecks += uint64(len(model.cases))
		specPass, _ := specSatisfied(model, vector)
		if !specPass {
			return nil
		}
		testsPass, failedTest, err := evaluateSuite(model, vector)
		if err != nil {
			return err
		}
		if testsPass || result.Witness != nil {
			return nil
		}
		highlight := fairnessHighlight(model, failedTest)
		requirement := &highlight.requirements[0]
		provenance := requirement.Provenance
		if failedTest != nil {
			provenance = failedTest.Provenance
		}
		result.Witness = makeCounterexample(semanticir.ObligationTestsComplete, model, vector, highlight, requirement, false, provenance)
		return nil
	})
	if err != nil {
		return result, err
	}
	finishObligation(&result)
	return result, nil
}

func newObligation(obligation semanticir.ProofObligation, model *finiteModel) ObligationResult {
	return ObligationResult{
		Obligation:     obligation,
		Verdict:        VerdictVerified,
		Method:         enumerationMethod,
		Exhaustive:     true,
		ReachableCases: uint64(len(model.cases)),
	}
}

func finishObligation(result *ObligationResult) {
	if result.Witness != nil {
		result.Verdict = VerdictNotVerified
	}
}

func enumerateBehaviorVectors(ctx context.Context, model *finiteModel, domains [][]string, visit func(behaviorVector) error) error {
	if len(domains) != len(model.cases) {
		return fmt.Errorf("internal universe mismatch: %d outcome domains for %d behavior variables", len(domains), len(model.cases))
	}
	product := uint64(1)
	for i, domain := range domains {
		if len(domain) == 0 {
			return fmt.Errorf("behavior variable %d has an empty outcome domain", i)
		}
		if product > math.MaxUint64/uint64(len(domain)) {
			return fmt.Errorf("finite behavior-vector count exceeds proof accounting capacity")
		}
		product *= uint64(len(domain))
	}

	vector := make(behaviorVector, len(model.cases))
	var visited uint64
	var walk func(int) error
	walk = func(index int) error {
		if index == len(model.cases) {
			if err := ctx.Err(); err != nil {
				return err
			}
			visited++
			return visit(vector)
		}
		finiteCase := model.cases[index]
		key := finiteCaseKey(model, finiteCase)
		for _, outcomeID := range domains[index] {
			vector[key] = outcomeID
			if err := walk(index + 1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if visited != product {
		return fmt.Errorf("incomplete finite enumeration: visited %d of %d behavior vectors", visited, product)
	}
	return nil
}

type specViolation struct {
	finiteCase  *finiteCase
	requirement *semanticir.RequirementCase
}

func specSatisfied(model *finiteModel, vector behaviorVector) (bool, *specViolation) {
	for i := range model.cases {
		finiteCase := &model.cases[i]
		key := finiteCaseKey(model, *finiteCase)
		outcomeID := vector[key]
		if !containsString(finiteCase.allowed, outcomeID) {
			if requirementID := finiteCase.rejectedBy[outcomeID]; requirementID != "" {
				for j := range finiteCase.requirements {
					if finiteCase.requirements[j].ID == requirementID {
						return false, &specViolation{finiteCase: finiteCase, requirement: &finiteCase.requirements[j]}
					}
				}
			}
			outcome := model.outcomes[outcomeID]
			for j := range finiteCase.requirements {
				requirement := &finiteCase.requirements[j]
				if !containsString(requirement.RequiredOutcomes, outcomeID) || !outcomeSatisfiesEffects(outcome, requirement.Effects) {
					return false, &specViolation{finiteCase: finiteCase, requirement: requirement}
				}
			}
			return false, &specViolation{finiteCase: finiteCase, requirement: &finiteCase.requirements[0]}
		}
	}
	return true, nil
}

func uniformDomains(model *finiteModel) [][]string {
	domains := make([][]string, len(model.cases))
	for i := range model.cases {
		domains[i] = append([]string(nil), model.operationOutcomes[model.cases[i].operation]...)
	}
	return domains
}

func fairnessHighlight(model *finiteModel, failedTest *semanticir.TestModel) *finiteCase {
	if failedTest != nil {
		refs := predicateBehaviorRefs(failedTest.Predicate)
		for _, ref := range refs {
			key := concreteCaseKey(ref.OperationID, model.operationDomains[ref.OperationID], ref.Conditions, ref.Inputs)
			for i := range model.cases {
				candidate := &model.cases[i]
				candidateKey := finiteCaseKey(model, *candidate)
				if candidateKey == key {
					return candidate
				}
			}
		}
	}
	return &model.cases[0]
}

func makeCounterexample(obligation semanticir.ProofObligation, model *finiteModel, vector behaviorVector, highlight *finiteCase, requirement *semanticir.RequirementCase, testPasses bool, provenance semanticir.Provenance) *semanticir.Counterexample {
	choices := make([]semanticir.BehaviorChoice, 0, len(model.cases))
	observed := make([]string, 0, len(model.cases))
	for _, finiteCase := range model.cases {
		key := finiteCaseKey(model, finiteCase)
		outcomeID := vector[key]
		choices = append(choices, semanticir.BehaviorChoice{
			Behavior: semanticir.BehaviorRef{
				OperationID: finiteCase.operation,
				Conditions:  cloneAssignment(finiteCase.conditions),
				Inputs:      cloneInputs(finiteCase.inputs),
				Provenance:  finiteCase.requirements[0].Provenance,
			},
			OutcomeID: outcomeID,
		})
		observed = append(observed, outcomeID)
	}
	expected := append([]string(nil), highlight.allowed...)
	canonical, _ := json.Marshal(struct {
		Obligation semanticir.ProofObligation  `json:"obligation"`
		Choices    []semanticir.BehaviorChoice `json:"choices"`
		Expected   []string                    `json:"expected"`
	}{obligation, choices, expected})
	digest := sha256.Sum256(canonical)
	return &semanticir.Counterexample{
		ID:               "cex-" + hex.EncodeToString(digest[:8]),
		Obligation:       obligation,
		Conditions:       cloneAssignment(highlight.conditions),
		OperationID:      highlight.operation,
		RequirementID:    requirement.ID,
		Choices:          choices,
		ObservedOutcomes: observed,
		ExpectedOutcomes: expected,
		TestPasses:       testPasses,
		Provenance:       provenance,
	}
}

func sortedUnique(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func blockedResult(blockers []Blocker, transcript Transcript) Result {
	transcript.Complete = false
	method := transcript.Method
	if method == "" {
		method = enumerationMethod
	}
	makeObligation := func(obligation semanticir.ProofObligation) ObligationResult {
		return ObligationResult{
			Obligation:     obligation,
			Verdict:        VerdictProofBlocked,
			Blockers:       append([]Blocker(nil), blockers...),
			Method:         method,
			Exhaustive:     false,
			ReachableCases: transcript.ReachableCases,
		}
	}
	return Result{
		Verdict:             VerdictProofBlocked,
		Reference:           makeObligation(semanticir.ObligationReferenceCorrectness),
		FalsePositive:       makeObligation(semanticir.ObligationTestsSound),
		Fairness:            makeObligation(semanticir.ObligationTestsComplete),
		ReferenceAcceptance: makeObligation(semanticir.ObligationReferenceAcceptance),
		Blockers:            append([]Blocker(nil), blockers...),
		Transcript:          transcript,
	}
}
