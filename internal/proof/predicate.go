package proof

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func (v *validator) predicateProvenance(predicate *semanticir.TestPredicate, label string) {
	if predicate == nil || predicate.Kind == "" {
		return
	}
	v.provenance(predicate.Provenance, label)
	v.requireProvenanceKind(predicate.Provenance, semanticir.ArtifactTests, label)
	if predicate.Observe != nil {
		v.provenance(predicate.Observe.Provenance, label+" observation")
		v.requireProvenanceKind(predicate.Observe.Provenance, semanticir.ArtifactTests, label+" observation")
		v.provenance(predicate.Observe.Behavior.Provenance, label+" observation behavior")
		v.requireProvenanceKind(predicate.Observe.Behavior.Provenance, semanticir.ArtifactTests, label+" observation behavior")
		if predicate.Observe.EffectValue != nil {
			v.expressionProvenance(predicate.Observe.EffectValue, label+" observation effect value")
			v.requireExpressionKind(predicate.Observe.EffectValue, semanticir.ArtifactTests, label+" observation effect value")
		}
	}
	if predicate.Left != nil {
		v.provenance(predicate.Left.Provenance, label+" left behavior")
		v.requireProvenanceKind(predicate.Left.Provenance, semanticir.ArtifactTests, label+" left behavior")
	}
	if predicate.Right != nil {
		v.provenance(predicate.Right.Provenance, label+" right behavior")
		v.requireProvenanceKind(predicate.Right.Provenance, semanticir.ArtifactTests, label+" right behavior")
	}
	for i := range predicate.Children {
		v.predicateProvenance(&predicate.Children[i], fmt.Sprintf("%s child %d", label, i))
	}
}

func (v *validator) validatePredicate(predicate semanticir.TestPredicate, label string, constraints map[string]map[string]semanticir.Constraint, targetOperations map[string]bool) {
	childCount := len(predicate.Children)
	switch predicate.Kind {
	case semanticir.PredicateTrue, semanticir.PredicateFalse:
		if childCount != 0 || predicate.Observe != nil || predicate.Left != nil || predicate.Right != nil {
			v.add("invalid-test-predicate", fmt.Sprintf("%s %s predicate carries operands", label, predicate.Kind), &predicate.Provenance)
		}
	case semanticir.PredicateAnd, semanticir.PredicateOr:
		if childCount < 2 || predicate.Observe != nil || predicate.Left != nil || predicate.Right != nil {
			v.add("invalid-test-predicate", fmt.Sprintf("%s %s predicate must have at least two children and no leaf operands", label, predicate.Kind), &predicate.Provenance)
		}
	case semanticir.PredicateNot:
		if childCount != 1 || predicate.Observe != nil || predicate.Left != nil || predicate.Right != nil {
			v.add("invalid-test-predicate", label+" not predicate must have exactly one child", &predicate.Provenance)
		}
	case semanticir.PredicateOutcomeIn:
		if childCount != 0 || predicate.Observe == nil || predicate.Left != nil || predicate.Right != nil {
			v.add("invalid-test-predicate", label+" outcome-in predicate must have exactly one observation", &predicate.Provenance)
		} else {
			observation := predicate.Observe
			if observation.Kind != semanticir.ObserveOutcome {
				v.add("invalid-test-predicate", label+" outcome-in predicate does not observe an outcome", &observation.Provenance)
			}
			v.validateLocalOutcomeIDs(observation.Behavior.OperationID, observation.OutcomeIDs, label+" accepted outcomes", false, &observation.Provenance)
			v.validateBehaviorRef(observation.Behavior, label+" observation", constraints, targetOperations)
		}
	case semanticir.PredicateOutcomeEqual:
		if childCount != 0 || predicate.Observe != nil || predicate.Left == nil || predicate.Right == nil {
			v.add("invalid-test-predicate", label+" outcome-equal predicate must have left and right behavior references", &predicate.Provenance)
		} else {
			v.validateBehaviorRef(*predicate.Left, label+" left", constraints, targetOperations)
			v.validateBehaviorRef(*predicate.Right, label+" right", constraints, targetOperations)
		}
	case semanticir.PredicateRaises:
		if childCount != 0 || predicate.Observe == nil || predicate.Left != nil || predicate.Right != nil {
			v.add("invalid-test-predicate", label+" raises predicate must have exactly one observation", &predicate.Provenance)
		} else {
			observation := predicate.Observe
			if observation.Kind != semanticir.ObserveRaise || strings.TrimSpace(observation.ExceptionType) == "" {
				v.add("invalid-test-predicate", label+" raises observation is missing a typed exception", &observation.Provenance)
			}
			v.validateBehaviorRef(observation.Behavior, label+" observation", constraints, targetOperations)
		}
	case semanticir.PredicateHasEffect:
		if childCount != 0 || predicate.Observe == nil || predicate.Left != nil || predicate.Right != nil {
			v.add("invalid-test-predicate", label+" has-effect predicate must have exactly one observation", &predicate.Provenance)
		} else {
			observation := predicate.Observe
			if observation.Kind != semanticir.ObserveEffect || strings.TrimSpace(observation.EffectTarget) == "" || !validEffectKind(observation.EffectKind) {
				v.add("invalid-test-predicate", label+" effect observation is missing a typed kind or target", &observation.Provenance)
			}
			if observation.EffectValue != nil {
				if _, err := evaluateExpression(*observation.EffectValue, nil); err != nil {
					v.add("unsupported-effect-value", fmt.Sprintf("%s expected effect value cannot be evaluated exactly: %v", label, err), &observation.Provenance)
				}
			}
			v.validateBehaviorRef(observation.Behavior, label+" observation", constraints, targetOperations)
		}
	default:
		v.add("unsupported-test-predicate", fmt.Sprintf("%s has unsupported predicate kind %q", label, predicate.Kind), &predicate.Provenance)
	}
	for i, child := range predicate.Children {
		v.validatePredicate(child, fmt.Sprintf("%s child %d", label, i), constraints, targetOperations)
	}
}

func (v *validator) validateBehaviorRef(ref semanticir.BehaviorRef, label string, constraints map[string]map[string]semanticir.Constraint, targetOperations map[string]bool) {
	if _, ok := v.semanticCaseKey(ref.OperationID, ref.Conditions, label, &ref.Provenance, constraints); ok {
		targetOperations[ref.OperationID] = true
	}
}

func (v *validator) validateAcceptedShortcut(test semanticir.TestModel) {
	if len(test.AcceptedOutcomes) == 0 {
		return
	}
	accepted := v.validateReferencedOutcomes(test.AcceptedOutcomes, "test model "+strconv.Quote(test.ID)+" accepted-outcome shortcut", true, &test.Provenance)
	predicate := test.Predicate
	if predicate.Kind != semanticir.PredicateOutcomeIn || predicate.Observe == nil || predicate.Observe.Kind != semanticir.ObserveOutcome {
		v.add("conflicting-test-shortcut", fmt.Sprintf("test model %q carries unary AcceptedOutcomes beside a non-unary predicate", test.ID), &test.Provenance)
		return
	}
	observed := predicate.Observe.Behavior
	domainIDs := v.operationDomains[observed.OperationID]
	keyA, okA := v.assignmentKeyFor(domainIDs, observed.Conditions, "test shortcut observation", &test.Provenance)
	keyB, okB := v.assignmentKeyFor(domainIDs, test.Conditions, "test shortcut selector", &test.Provenance)
	if !okA || !okB || observed.OperationID != test.OperationID || keyA != keyB {
		v.add("conflicting-test-shortcut", fmt.Sprintf("test model %q shortcut selects a different behavior than its predicate", test.ID), &test.Provenance)
		return
	}
	predicateSet := make(map[string]bool)
	for _, id := range predicate.Observe.OutcomeIDs {
		predicateSet[id] = true
	}
	if len(accepted) != len(predicateSet) {
		v.add("conflicting-test-shortcut", fmt.Sprintf("test model %q shortcut differs from its predicate", test.ID), &test.Provenance)
		return
	}
	for id := range accepted {
		if !predicateSet[id] {
			v.add("conflicting-test-shortcut", fmt.Sprintf("test model %q shortcut differs from its predicate", test.ID), &test.Provenance)
			return
		}
	}
}

func validEffectKind(kind semanticir.EffectKind) bool {
	switch kind {
	case semanticir.EffectRead, semanticir.EffectWrite, semanticir.EffectCall, semanticir.EffectOutput:
		return true
	default:
		return false
	}
}

type behaviorVector map[caseKey]string

func evaluateSuite(model *finiteModel, vector behaviorVector) (bool, *semanticir.TestModel, error) {
	for i := range model.tests {
		passed, err := evaluatePredicate(model, model.tests[i].Predicate, vector)
		if err != nil {
			return false, &model.tests[i], err
		}
		if !passed {
			return false, &model.tests[i], nil
		}
	}
	return true, nil, nil
}

func evaluatePredicate(model *finiteModel, predicate semanticir.TestPredicate, vector behaviorVector) (bool, error) {
	switch predicate.Kind {
	case semanticir.PredicateTrue:
		return true, nil
	case semanticir.PredicateFalse:
		return false, nil
	case semanticir.PredicateAnd:
		for _, child := range predicate.Children {
			value, err := evaluatePredicate(model, child, vector)
			if err != nil || !value {
				return value, err
			}
		}
		return true, nil
	case semanticir.PredicateOr:
		for _, child := range predicate.Children {
			value, err := evaluatePredicate(model, child, vector)
			if err != nil {
				return false, err
			}
			if value {
				return true, nil
			}
		}
		return false, nil
	case semanticir.PredicateNot:
		value, err := evaluatePredicate(model, predicate.Children[0], vector)
		return !value, err
	case semanticir.PredicateOutcomeIn:
		outcomeID, _, err := observedOutcome(model, predicate.Observe.Behavior, vector)
		if err != nil {
			return false, err
		}
		for _, accepted := range predicate.Observe.OutcomeIDs {
			if accepted == outcomeID {
				return true, nil
			}
		}
		return false, nil
	case semanticir.PredicateOutcomeEqual:
		left, _, err := observedOutcome(model, *predicate.Left, vector)
		if err != nil {
			return false, err
		}
		right, _, err := observedOutcome(model, *predicate.Right, vector)
		if err != nil {
			return false, err
		}
		return left == right, nil
	case semanticir.PredicateRaises:
		_, outcome, err := observedOutcome(model, predicate.Observe.Behavior, vector)
		if err != nil {
			return false, err
		}
		if outcome.Kind != semanticir.OutcomeRaise || outcome.ExceptionType != predicate.Observe.ExceptionType {
			return false, nil
		}
		return predicate.Observe.Message == "" || outcome.Message == predicate.Observe.Message, nil
	case semanticir.PredicateHasEffect:
		_, outcome, err := observedOutcome(model, predicate.Observe.Behavior, vector)
		if err != nil {
			return false, err
		}
		for _, effect := range outcome.Effects {
			if effect.Kind != predicate.Observe.EffectKind || effect.Target != predicate.Observe.EffectTarget {
				continue
			}
			if predicate.Observe.EffectValue == nil {
				return true, nil
			}
			if effect.Value == nil {
				continue
			}
			expected, expectedErr := evaluateExpression(*predicate.Observe.EffectValue, nil)
			actual, actualErr := evaluateExpression(*effect.Value, nil)
			if expectedErr != nil || actualErr != nil {
				return false, fmt.Errorf("evaluate effect value: expected=%v actual=%v", expectedErr, actualErr)
			}
			if reflect.DeepEqual(expected, actual) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported test predicate %q", predicate.Kind)
	}
}

func observedOutcome(model *finiteModel, ref semanticir.BehaviorRef, vector behaviorVector) (string, semanticir.ObservableOutcome, error) {
	key := concreteCaseKey(ref.OperationID, model.operationDomains[ref.OperationID], ref.Conditions, ref.Inputs)
	outcomeID, exists := vector[key]
	if !exists {
		return "", semanticir.ObservableOutcome{}, fmt.Errorf("predicate references missing behavior %q %s", ref.OperationID, key.assignment)
	}
	outcome, exists := model.outcomes[outcomeID]
	if !exists {
		return "", semanticir.ObservableOutcome{}, fmt.Errorf("behavior selects undeclared outcome %q", outcomeID)
	}
	return outcomeID, outcome, nil
}

func predicateBehaviorRefs(predicate semanticir.TestPredicate) []semanticir.BehaviorRef {
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
		refs = append(refs, predicateBehaviorRefs(child)...)
	}
	sort.SliceStable(refs, func(i, j int) bool {
		return semanticir.BehaviorRefKey(refs[i]) < semanticir.BehaviorRefKey(refs[j])
	})
	return refs
}

func canonicalAssignment(domainIDs []string, assignment semanticir.Assignment) string {
	var builder strings.Builder
	for _, domainID := range domainIDs {
		builder.WriteString(strconv.Quote(domainID))
		builder.WriteByte('=')
		builder.WriteString(strconv.Quote(assignment[domainID]))
		builder.WriteByte(';')
	}
	return builder.String()
}

func canonicalAssignmentFromMap(assignment semanticir.Assignment) string {
	ids := make([]string, 0, len(assignment))
	for id := range assignment {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return canonicalAssignment(ids, assignment)
}
