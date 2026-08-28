package proof

import (
	"fmt"
	"sort"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// EvaluateTestPredicate evaluates one global predicate against one complete
// finite behavior vector. It is intentionally independent of Task.TestSuite
// so the exhaustive test-IR producer can compare its separately translated
// predicate with each concrete verifier result without duplicating proof
// semantics.
func EvaluateTestPredicate(task *semanticir.Task, predicate semanticir.TestPredicate, choices []semanticir.BehaviorChoice) (bool, error) {
	if task == nil {
		return false, fmt.Errorf("task and test predicate are required")
	}
	model, expected, err := predicateEvaluationModel(task)
	if err != nil {
		return false, err
	}
	if err := validateEvaluatedPredicate(model, predicate); err != nil {
		return false, err
	}
	vector := make(behaviorVector, len(choices))
	for index, choice := range choices {
		domainIDs, exists := model.operationDomains[choice.Behavior.OperationID]
		if !exists {
			return false, fmt.Errorf("choice %d names unknown behavior operation %q", index, choice.Behavior.OperationID)
		}
		key, err := evaluationAssignmentKey(task, choice.Behavior.OperationID, domainIDs, choice.Behavior.Conditions)
		if err != nil {
			return false, fmt.Errorf("choice %d: %w", index, err)
		}
		if choice.Behavior.Inputs == nil {
			return false, fmt.Errorf("choice %d names a semantic category instead of one concrete behavior point", index)
		}
		caseID := caseKey{operation: choice.Behavior.OperationID, assignment: key, inputs: inputPointKey(choice.Behavior.Inputs)}
		if !expected[caseID] {
			return false, fmt.Errorf("choice %d selects constrained or undeclared behavior", index)
		}
		if _, duplicate := vector[caseID]; duplicate {
			return false, fmt.Errorf("choice %d duplicates behavior %q %s", index, caseID.operation, caseID.assignment)
		}
		if !containsString(model.operationOutcomes[caseID.operation], choice.OutcomeID) {
			return false, fmt.Errorf("choice %d selects outcome %q outside operation %q", index, choice.OutcomeID, caseID.operation)
		}
		vector[caseID] = choice.OutcomeID
	}
	if len(vector) != len(expected) {
		return false, fmt.Errorf("behavior vector has %d choices; want exactly %d reachable behaviors", len(vector), len(expected))
	}
	return evaluatePredicate(model, predicate, vector)
}

func predicateEvaluationModel(task *semanticir.Task) (*finiteModel, map[caseKey]bool, error) {
	points, pointDiagnostics := semanticir.ConcreteBehaviorPoints(task)
	for _, diagnostic := range pointDiagnostics {
		if diagnostic.Severity == semanticir.SeverityError {
			return nil, nil, fmt.Errorf("concrete point universe: %s: %s", diagnostic.Code, diagnostic.Message)
		}
	}
	domainValues := make(map[string][]string, len(task.Domains))
	for _, domain := range task.Domains {
		if strings.TrimSpace(domain.ID) == "" || len(domain.Values) == 0 {
			return nil, nil, fmt.Errorf("domain %q is empty or unnamed", domain.ID)
		}
		if _, duplicate := domainValues[domain.ID]; duplicate {
			return nil, nil, fmt.Errorf("domain %q is duplicated", domain.ID)
		}
		seen := map[string]bool{}
		for _, value := range domain.Values {
			if strings.TrimSpace(value.ID) == "" || seen[value.ID] {
				return nil, nil, fmt.Errorf("domain %q has an empty or duplicate value %q", domain.ID, value.ID)
			}
			seen[value.ID] = true
			domainValues[domain.ID] = append(domainValues[domain.ID], value.ID)
		}
		sort.Strings(domainValues[domain.ID])
	}
	outcomes := make(map[string]semanticir.ObservableOutcome, len(task.Outcomes))
	for _, outcome := range task.Outcomes {
		if strings.TrimSpace(outcome.ID) == "" {
			return nil, nil, fmt.Errorf("observable outcome has an empty ID")
		}
		if _, duplicate := outcomes[outcome.ID]; duplicate {
			return nil, nil, fmt.Errorf("outcome %q is duplicated", outcome.ID)
		}
		outcomes[outcome.ID] = outcome
	}
	model := &finiteModel{
		operationDomains: make(map[string][]string), operationOutcomes: make(map[string][]string),
		outcomes: outcomes, reachable: make(map[string][]semanticir.Assignment),
	}
	operations := make(map[string]semanticir.Operation)
	for _, operation := range task.Operations {
		if operation.Kind == semanticir.OperationTest {
			continue
		}
		if strings.TrimSpace(operation.ID) == "" {
			return nil, nil, fmt.Errorf("behavior operation has an empty ID")
		}
		if _, duplicate := operations[operation.ID]; duplicate {
			return nil, nil, fmt.Errorf("operation %q is duplicated", operation.ID)
		}
		operations[operation.ID] = operation
		domainIDs := append([]string(nil), operation.DomainIDs...)
		sort.Strings(domainIDs)
		for index, domainID := range domainIDs {
			if _, exists := domainValues[domainID]; !exists || (index > 0 && domainID == domainIDs[index-1]) {
				return nil, nil, fmt.Errorf("operation %q has missing or duplicate domain %q", operation.ID, domainID)
			}
		}
		model.operationDomains[operation.ID] = domainIDs
		seenOutcome := map[string]bool{}
		for _, outcomeID := range operation.OutcomeIDs {
			if _, exists := outcomes[outcomeID]; !exists || seenOutcome[outcomeID] {
				return nil, nil, fmt.Errorf("operation %q has missing or duplicate outcome %q", operation.ID, outcomeID)
			}
			seenOutcome[outcomeID] = true
			model.operationOutcomes[operation.ID] = append(model.operationOutcomes[operation.ID], outcomeID)
		}
		if len(model.operationOutcomes[operation.ID]) == 0 {
			return nil, nil, fmt.Errorf("operation %q has no finite outcome universe", operation.ID)
		}
		sort.Strings(model.operationOutcomes[operation.ID])
	}
	if len(operations) == 0 {
		return nil, nil, fmt.Errorf("task has no behavior operations")
	}
	expected := make(map[caseKey]bool)
	seenCategories := make(map[caseKey]bool)
	for index, point := range points {
		operation, exists := operations[point.OperationID]
		if !exists {
			return nil, nil, fmt.Errorf("concrete point %d names unknown operation %q", index, point.OperationID)
		}
		domainIDs := model.operationDomains[operation.ID]
		assignmentKey, err := evaluationAssignmentKey(task, operation.ID, domainIDs, point.Conditions)
		if err != nil {
			return nil, nil, fmt.Errorf("concrete point %d: %w", index, err)
		}
		finiteCase := finiteCase{operation: operation.ID, conditions: cloneAssignment(point.Conditions), inputs: cloneInputs(point.Inputs)}
		pointKey := finiteCaseKey(model, finiteCase)
		if expected[pointKey] {
			return nil, nil, fmt.Errorf("concrete point universe repeats %q", semanticir.BehaviorRefKey(point))
		}
		expected[pointKey] = true
		model.cases = append(model.cases, finiteCase)
		categoryKey := caseKey{operation: operation.ID, assignment: assignmentKey}
		if !seenCategories[categoryKey] {
			seenCategories[categoryKey] = true
			model.reachable[operation.ID] = append(model.reachable[operation.ID], cloneAssignment(point.Conditions))
		}
	}
	return model, expected, nil
}

func evaluationAssignmentKey(task *semanticir.Task, operationID string, domainIDs []string, assignment semanticir.Assignment) (string, error) {
	if len(assignment) != len(domainIDs) {
		return "", fmt.Errorf("operation %q assignment fixes %d domains; want %d", operationID, len(assignment), len(domainIDs))
	}
	for _, domainID := range domainIDs {
		valueID, exists := assignment[domainID]
		if !exists {
			return "", fmt.Errorf("operation %q assignment omits domain %q", operationID, domainID)
		}
		found := false
		for _, domain := range task.Domains {
			if domain.ID != domainID {
				continue
			}
			for _, value := range domain.Values {
				found = found || value.ID == valueID
			}
		}
		if !found {
			return "", fmt.Errorf("operation %q assignment uses unknown %s=%q", operationID, domainID, valueID)
		}
	}
	return canonicalAssignment(domainIDs, assignment), nil
}

func validateEvaluatedPredicate(model *finiteModel, predicate semanticir.TestPredicate) error {
	leafEmpty := predicate.Observe == nil && predicate.Left == nil && predicate.Right == nil
	switch predicate.Kind {
	case semanticir.PredicateTrue, semanticir.PredicateFalse:
		if len(predicate.Children) != 0 || !leafEmpty {
			return fmt.Errorf("%s predicate carries operands", predicate.Kind)
		}
	case semanticir.PredicateAnd, semanticir.PredicateOr:
		if len(predicate.Children) < 2 || !leafEmpty {
			return fmt.Errorf("%s predicate requires at least two children", predicate.Kind)
		}
	case semanticir.PredicateNot:
		if len(predicate.Children) != 1 || !leafEmpty {
			return fmt.Errorf("not predicate requires exactly one child")
		}
	case semanticir.PredicateOutcomeIn, semanticir.PredicateRaises, semanticir.PredicateHasEffect:
		if len(predicate.Children) != 0 || predicate.Observe == nil || predicate.Left != nil || predicate.Right != nil {
			return fmt.Errorf("%s predicate requires exactly one observation", predicate.Kind)
		}
		wantKind := map[semanticir.TestPredicateKind]semanticir.ObservationKind{
			semanticir.PredicateOutcomeIn: semanticir.ObserveOutcome,
			semanticir.PredicateRaises:    semanticir.ObserveRaise,
			semanticir.PredicateHasEffect: semanticir.ObserveEffect,
		}[predicate.Kind]
		if predicate.Observe.Kind != wantKind {
			return fmt.Errorf("%s predicate has observation kind %q", predicate.Kind, predicate.Observe.Kind)
		}
		if err := validateEvaluatedBehaviorRef(model, predicate.Observe.Behavior); err != nil {
			return err
		}
		switch predicate.Kind {
		case semanticir.PredicateOutcomeIn:
			if len(predicate.Observe.OutcomeIDs) == 0 {
				return fmt.Errorf("outcome-in predicate has an empty outcome set")
			}
			if predicate.Observe.ExceptionType != "" || predicate.Observe.Message != "" || predicate.Observe.EffectKind != "" || predicate.Observe.EffectTarget != "" || predicate.Observe.EffectValue != nil {
				return fmt.Errorf("outcome-in predicate also carries raise/effect fields")
			}
			seen := map[string]bool{}
			for _, outcomeID := range predicate.Observe.OutcomeIDs {
				if seen[outcomeID] || !containsString(model.operationOutcomes[predicate.Observe.Behavior.OperationID], outcomeID) {
					return fmt.Errorf("outcome-in predicate has duplicate or non-local outcome %q", outcomeID)
				}
				seen[outcomeID] = true
			}
		case semanticir.PredicateRaises:
			if strings.TrimSpace(predicate.Observe.ExceptionType) == "" || len(predicate.Observe.OutcomeIDs) != 0 || predicate.Observe.EffectKind != "" || predicate.Observe.EffectTarget != "" || predicate.Observe.EffectValue != nil {
				return fmt.Errorf("raises predicate must set only exception type/message")
			}
		case semanticir.PredicateHasEffect:
			if !validEffectKind(predicate.Observe.EffectKind) || strings.TrimSpace(predicate.Observe.EffectTarget) == "" || len(predicate.Observe.OutcomeIDs) != 0 || predicate.Observe.ExceptionType != "" || predicate.Observe.Message != "" {
				return fmt.Errorf("has-effect predicate must set only a valid effect kind/target and optional value")
			}
			if predicate.Observe.EffectValue != nil {
				if _, err := evaluateExpression(*predicate.Observe.EffectValue, nil); err != nil {
					return fmt.Errorf("has-effect value cannot be evaluated exactly: %w", err)
				}
			}
		}
	case semanticir.PredicateOutcomeEqual:
		if len(predicate.Children) != 0 || predicate.Observe != nil || predicate.Left == nil || predicate.Right == nil {
			return fmt.Errorf("outcome-equal predicate requires left and right behaviors")
		}
		if err := validateEvaluatedBehaviorRef(model, *predicate.Left); err != nil {
			return err
		}
		if err := validateEvaluatedBehaviorRef(model, *predicate.Right); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported test predicate %q", predicate.Kind)
	}
	for _, child := range predicate.Children {
		if err := validateEvaluatedPredicate(model, child); err != nil {
			return err
		}
	}
	return nil
}

func validateEvaluatedBehaviorRef(model *finiteModel, ref semanticir.BehaviorRef) error {
	domainIDs, exists := model.operationDomains[ref.OperationID]
	if !exists {
		return fmt.Errorf("predicate refers to unknown operation %q", ref.OperationID)
	}
	if ref.Inputs == nil {
		return fmt.Errorf("predicate refers to semantic category %q without exact concrete inputs", ref.OperationID)
	}
	key := concreteCaseKey(ref.OperationID, domainIDs, ref.Conditions, ref.Inputs)
	for _, finiteCase := range model.cases {
		if finiteCaseKey(model, finiteCase) == key && len(ref.Conditions) == len(domainIDs) {
			return nil
		}
	}
	return fmt.Errorf("predicate refers to constrained, incomplete, or undeclared behavior %q %s", ref.OperationID, key.assignment)
}
