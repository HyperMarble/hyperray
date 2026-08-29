package pipeline

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/HyperMarble/hyperray/internal/executor"
	"github.com/HyperMarble/hyperray/internal/proof"
	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// witnessConfirmationRequest binds T(C) and every SAT witness to the same
// independently compiled Spec/reference/Test/environment/proof context.
func witnessConfirmationRequest(ctx context.Context, task *semanticir.Task, records []translationRecord, frozen executor.FrozenWitnessContext, proofResult proof.Result) (executor.WitnessConfirmationRequest, []string) {
	referenceChoices, err := referenceChoices(task)
	if err != nil {
		return executor.WitnessConfirmationRequest{}, []string{err.Error()}
	}
	referenceDigest, err := semanticir.Digest(referenceChoices)
	if err != nil {
		return executor.WitnessConfirmationRequest{}, []string{"digest exact reference vector: " + err.Error()}
	}
	request := executor.WitnessConfirmationRequest{ReferenceAcceptance: executor.ReferenceAcceptancePlan{
		ID: task.ID + ":reference-acceptance", Context: frozen,
		ReferenceChoices: referenceChoices, ReferenceChoicesSHA256: referenceDigest,
	}}
	seen := map[string]bool{}
	for _, witness := range proofResult.Counterexamples {
		if witness.ID == "" || seen[witness.ID] {
			return executor.WitnessConfirmationRequest{}, []string{"proof returned an empty or duplicate witness ID"}
		}
		seen[witness.ID] = true
		if diagnostics := semanticir.ValidateCounterexample(task, witness); semanticir.HasErrors(diagnostics) {
			return executor.WitnessConfirmationRequest{}, diagnosticStrings(diagnostics)
		}
		// T(C)=false is confirmed by the mandatory clean reference acceptance
		// execution above, not by a synthetic second witness mechanism.
		if witness.Obligation == semanticir.ObligationReferenceAcceptance {
			if !sameBehaviorChoices(witness.Choices, referenceChoices) {
				return executor.WitnessConfirmationRequest{}, []string{"reference-acceptance witness differs from independently translated C"}
			}
			continue
		}
		plan := executor.WitnessPlan{ID: "witness:" + witness.ID, Context: frozen, Witness: witness}
		switch witness.Obligation {
		case semanticir.ObligationReferenceCorrectness:
			plan.Kind = executor.WitnessReference
			owner, err := counterexampleOwner(witness, records)
			if err != nil {
				return executor.WitnessConfirmationRequest{}, []string{fmt.Sprintf("reference witness %q: %v", witness.ID, err)}
			}
			record := records[owner]
			probe, diagnostics := dispatchGenerateProbe(ctx, semanticir.MaterializationRequest{
				Frontend: record.request, Task: task, Model: record.model, Counterexample: witness,
			})
			if semanticir.HasErrors(diagnostics) {
				return executor.WitnessConfirmationRequest{}, diagnosticStrings(diagnostics)
			}
			if probe.ID == "" || probe.WitnessID != witness.ID || !reflect.DeepEqual(probe.Witness, witness) || !reflect.DeepEqual(probe.Workspace, frozen.Workspace) {
				return executor.WitnessConfirmationRequest{}, []string{fmt.Sprintf("reference witness %q probe is empty or detached", witness.ID)}
			}
			plan.Probe = &probe
		case semanticir.ObligationTestsSound, semanticir.ObligationTestsComplete:
			if witness.Obligation == semanticir.ObligationTestsSound {
				plan.Kind = executor.WitnessFalsePositive
			} else {
				plan.Kind = executor.WitnessFalseNegative
			}
			edits, blockers := materializeBehaviorVector(ctx, task, records, witness)
			if len(blockers) != 0 {
				return executor.WitnessConfirmationRequest{}, blockers
			}
			plan.EditPlans = edits
		default:
			return executor.WitnessConfirmationRequest{}, []string{fmt.Sprintf("unsupported proof witness obligation %q", witness.Obligation)}
		}
		request.Witnesses = append(request.Witnesses, plan)
	}
	return request, nil
}

func materializeBehaviorVector(ctx context.Context, task *semanticir.Task, records []translationRecord, witness semanticir.Counterexample) ([]semanticir.EditPlan, []string) {
	changed := map[int]bool{}
	for _, choice := range witness.Choices {
		owner, behaviorCase, err := choiceOwner(choice, records)
		if err != nil {
			return nil, []string{fmt.Sprintf("counterexample %q: %v", witness.ID, err)}
		}
		if !contains(behaviorCase.OutcomeIDs, choice.OutcomeID) {
			changed[owner] = true
		}
	}
	if len(changed) == 0 {
		clean, err := referenceChoices(task)
		if err != nil || !sameBehaviorChoices(witness.Choices, clean) {
			return nil, []string{fmt.Sprintf("counterexample %q has no edits but differs from the clean reference vector", witness.ID)}
		}
		// Executor accepts an empty mechanism only for this exact semantic
		// identity and reuses the mandatory fresh T(C) evidence as a typed
		// baseline-vector confirmation. No no-op edit is manufactured.
		return nil, nil
	}
	owners := make([]int, 0, len(changed))
	for owner := range changed {
		owners = append(owners, owner)
	}
	sort.Ints(owners)
	want := semanticir.ExpectedSemantics{
		Conditions: cloneAssignment(witness.Conditions), OperationID: witness.OperationID,
		OutcomeIDs: append([]string(nil), witness.ObservedOutcomes...),
		Choices:    append([]semanticir.BehaviorChoice(nil), witness.Choices...), TestPasses: witness.TestPasses,
	}
	plans := make([]semanticir.EditPlan, 0, len(owners))
	for _, owner := range owners {
		record := records[owner]
		plan, diagnostics := dispatchMaterialize(ctx, semanticir.MaterializationRequest{
			Frontend: record.request, Task: task, Model: record.model, Counterexample: witness,
		})
		if semanticir.HasErrors(diagnostics) {
			return nil, diagnosticStrings(diagnostics)
		}
		if plan.ID == "" || plan.WitnessID != witness.ID || plan.Artifact != record.request.Artifact || len(plan.Edits) == 0 ||
			!reflect.DeepEqual(plan.Expected.Choices, witness.Choices) || plan.Expected.TestPasses != witness.TestPasses {
			return nil, []string{fmt.Sprintf("materializer returned an empty, stale, or detached plan for witness %q artifact %q", witness.ID, record.request.Artifact.ID)}
		}
		if plan.Provenance.ArtifactID != record.request.Artifact.ID || plan.Provenance.ArtifactDigest != record.request.Artifact.Digest || plan.Provenance.Translation != semanticir.TranslationTranslated {
			return nil, []string{fmt.Sprintf("materializer returned unbound provenance for witness %q artifact %q", witness.ID, record.request.Artifact.ID)}
		}
		if err := validateCombinedEdits(plan.Edits); err != nil {
			return nil, []string{fmt.Sprintf("counterexample %q artifact %q: %v", witness.ID, record.request.Artifact.ID, err)}
		}
		plan.ID = witness.ID + ":" + record.request.Artifact.ID
		plan.Expected = want
		plans = append(plans, plan)
	}
	return plans, nil
}

func referenceChoices(task *semanticir.Task) ([]semanticir.BehaviorChoice, error) {
	if task == nil || len(task.CodeCases) == 0 {
		return nil, fmt.Errorf("independently translated reference IR has no exact cases")
	}
	choices := make([]semanticir.BehaviorChoice, 0, len(task.CodeCases))
	for _, behaviorCase := range task.CodeCases {
		if len(behaviorCase.OutcomeIDs) != 1 || behaviorCase.OperationID == "" {
			return nil, fmt.Errorf("reference case %q is not one exact concrete behavior", behaviorCase.ID)
		}
		choices = append(choices, semanticir.BehaviorChoice{Behavior: semanticir.BehaviorRef{
			OperationID: behaviorCase.OperationID, Conditions: cloneAssignment(behaviorCase.Conditions), Inputs: cloneLiteralMap(behaviorCase.Inputs), Provenance: behaviorCase.Provenance,
		}, OutcomeID: behaviorCase.OutcomeIDs[0]})
	}
	sort.Slice(choices, func(i, j int) bool {
		left, _ := semanticir.Digest(choices[i])
		right, _ := semanticir.Digest(choices[j])
		return left < right
	})
	return choices, nil
}

func sameBehaviorChoices(left, right []semanticir.BehaviorChoice) bool {
	return reflect.DeepEqual(left, right)
}

func counterexampleOwner(counterexample semanticir.Counterexample, records []translationRecord) (int, error) {
	for _, choice := range counterexample.Choices {
		if choice.Behavior.OperationID == counterexample.OperationID && sameAssignment(choice.Behavior.Conditions, counterexample.Conditions) {
			owner, _, err := choiceOwner(choice, records)
			return owner, err
		}
	}
	return -1, fmt.Errorf("violating behavior component has no complete-vector choice")
}

func choiceOwner(choice semanticir.BehaviorChoice, records []translationRecord) (int, semanticir.BehaviorCase, error) {
	owner := -1
	var found semanticir.BehaviorCase
	for index, record := range records {
		if record.model.Kind != semanticir.ArtifactCode {
			continue
		}
		for _, behaviorCase := range record.model.Cases {
			if behaviorCase.OperationID == choice.Behavior.OperationID && sameAssignment(behaviorCase.Conditions, choice.Behavior.Conditions) {
				if owner != -1 {
					return -1, semanticir.BehaviorCase{}, fmt.Errorf("behavior choice %s is owned by multiple frozen code artifacts", choice.Behavior.OperationID)
				}
				owner, found = index, behaviorCase
			}
		}
	}
	if owner == -1 {
		return -1, semanticir.BehaviorCase{}, fmt.Errorf("behavior choice %s has no frozen code artifact", choice.Behavior.OperationID)
	}
	return owner, found, nil
}

func validateCombinedEdits(edits []semanticir.ByteRangeReplacement) error {
	if len(edits) == 0 {
		return fmt.Errorf("no exact byte edits")
	}
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].StartByte == edits[j].StartByte {
			return edits[i].EndByte < edits[j].EndByte
		}
		return edits[i].StartByte < edits[j].StartByte
	})
	for index := range edits {
		if edits[index].StartByte < 0 || edits[index].EndByte < edits[index].StartByte || len(edits[index].ExpectedBytes) != edits[index].EndByte-edits[index].StartByte {
			return fmt.Errorf("invalid edit range [%d,%d)", edits[index].StartByte, edits[index].EndByte)
		}
		if reflect.DeepEqual(edits[index].ExpectedBytes, edits[index].Replacement) {
			return fmt.Errorf("no-op edit range [%d,%d)", edits[index].StartByte, edits[index].EndByte)
		}
		if index > 0 && edits[index].StartByte < edits[index-1].EndByte {
			return fmt.Errorf("overlapping materializer edits")
		}
	}
	return nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func cloneAssignment(in semanticir.Assignment) semanticir.Assignment {
	out := make(semanticir.Assignment, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneLiteralMap(values map[string]semanticir.Literal) map[string]semanticir.Literal {
	if values == nil {
		return nil
	}
	cloned := make(map[string]semanticir.Literal, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
