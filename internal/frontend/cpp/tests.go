package cpp

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func (l *lowerer) buildTestModels() {
	if l.request.Kind != semanticir.ArtifactTests {
		return
	}
	for _, operation := range l.operations {
		assertions := make([]semanticir.Assertion, 0, len(operation.asserts))
		predicates := make([]semanticir.TestPredicate, 0, len(operation.asserts))
		for _, call := range operation.asserts {
			assertion, ok := l.lowerAssertion(call, operation.node)
			if !ok {
				continue
			}
			predicate, ok := l.predicateForAssertion(assertion)
			if !ok {
				l.blockProvenance(assertion.Provenance, "global-test-predicate", fmt.Sprintf("assertion %s cannot be expressed as an exact predicate over the candidate behavior vector", call.Name))
				continue
			}
			assertions = append(assertions, assertion)
			predicates = append(predicates, predicate)
		}
		if len(operation.asserts) == 0 {
			l.block(operation.node, "test-without-oracle", fmt.Sprintf("test operation %s has no supported observable assertion", operation.operation.ID), semanticir.DiagnosticIncomplete)
			continue
		}
		if len(predicates) != len(operation.asserts) {
			continue
		}
		predicate := semanticir.TestPredicate{Kind: semanticir.PredicateAnd, Children: predicates, Provenance: operation.operation.Provenance}
		if len(predicates) == 1 {
			predicate = predicates[0]
		}
		test := semanticir.TestModel{
			ID:               operation.operation.ID,
			Conditions:       semanticir.Assignment{},
			Assertions:       assertions,
			AcceptedOutcomes: []string{},
			Predicate:        predicate,
			Provenance:       operation.operation.Provenance,
		}
		if predicate.Observe != nil {
			test.OperationID = predicate.Observe.Behavior.OperationID
			test.Conditions = cloneAssignment(predicate.Observe.Behavior.Conditions)
			if predicate.Kind == semanticir.PredicateOutcomeIn {
				test.AcceptedOutcomes = append([]string(nil), predicate.Observe.OutcomeIDs...)
			}
		}
		l.tests = append(l.tests, test)
	}
}

func (l *lowerer) predicateForAssertion(assertion semanticir.Assertion) (semanticir.TestPredicate, bool) {
	switch assertion.Kind {
	case semanticir.AssertTrue:
		if assertion.Actual == nil {
			return semanticir.TestPredicate{}, false
		}
		return l.predicateForBoolExpression(*assertion.Actual)
	case semanticir.AssertFalse:
		if assertion.Actual == nil {
			return semanticir.TestPredicate{}, false
		}
		child, ok := l.predicateForBoolExpression(*assertion.Actual)
		if !ok {
			return semanticir.TestPredicate{}, false
		}
		return semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{child}, Provenance: assertion.Provenance}, true
	case semanticir.AssertEqual, semanticir.AssertNotEqual:
		if assertion.Actual == nil || assertion.Expected == nil {
			return semanticir.TestPredicate{}, false
		}
		predicate, ok := l.equalityPredicate(*assertion.Actual, *assertion.Expected, assertion.Provenance)
		if !ok {
			return semanticir.TestPredicate{}, false
		}
		if assertion.Kind == semanticir.AssertNotEqual {
			predicate = semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{predicate}, Provenance: assertion.Provenance}
		}
		return predicate, true
	case semanticir.AssertRaises:
		if assertion.Actual == nil || assertion.Actual.Kind != semanticir.ExprCall {
			return semanticir.TestPredicate{}, false
		}
		behavior, ok := l.behaviorRef(*assertion.Actual)
		if !ok {
			return semanticir.TestPredicate{}, false
		}
		observation := semanticir.Observation{Kind: semanticir.ObserveRaise, Behavior: behavior, ExceptionType: assertion.ExceptionType, Message: assertion.Message, OutcomeIDs: []string{}, Provenance: assertion.Provenance}
		return semanticir.TestPredicate{Kind: semanticir.PredicateRaises, Observe: &observation, Provenance: assertion.Provenance}, true
	default:
		return semanticir.TestPredicate{}, false
	}
}

func (l *lowerer) predicateForBoolExpression(expression semanticir.Expression) (semanticir.TestPredicate, bool) {
	if expression.Kind == semanticir.ExprCall && expression.Type == semanticir.TypeBool {
		literal := semanticir.Literal{Type: semanticir.TypeBool, Bool: true}
		return l.callOutcomePredicate(expression, literal, expression.Provenance)
	}
	if expression.Kind == semanticir.ExprUnary && expression.Operator == semanticir.OpNot && len(expression.Operands) == 1 {
		child, ok := l.predicateForBoolExpression(expression.Operands[0])
		if !ok {
			return semanticir.TestPredicate{}, false
		}
		return semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{child}, Provenance: expression.Provenance}, true
	}
	if expression.Kind == semanticir.ExprBool && len(expression.Operands) == 2 {
		left, okLeft := l.predicateForBoolExpression(expression.Operands[0])
		right, okRight := l.predicateForBoolExpression(expression.Operands[1])
		if !okLeft || !okRight {
			return semanticir.TestPredicate{}, false
		}
		kind := semanticir.PredicateAnd
		if expression.Operator == semanticir.OpOr {
			kind = semanticir.PredicateOr
		} else if expression.Operator != semanticir.OpAnd {
			return semanticir.TestPredicate{}, false
		}
		return semanticir.TestPredicate{Kind: kind, Children: []semanticir.TestPredicate{left, right}, Provenance: expression.Provenance}, true
	}
	if expression.Kind == semanticir.ExprCompare && len(expression.Operands) == 2 && (expression.Operator == semanticir.OpEQ || expression.Operator == semanticir.OpNE) {
		predicate, ok := l.equalityPredicate(expression.Operands[0], expression.Operands[1], expression.Provenance)
		if !ok {
			return semanticir.TestPredicate{}, false
		}
		if expression.Operator == semanticir.OpNE {
			predicate = semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{predicate}, Provenance: expression.Provenance}
		}
		return predicate, true
	}
	return semanticir.TestPredicate{}, false
}

func (l *lowerer) equalityPredicate(left, right semanticir.Expression, provenance semanticir.Provenance) (semanticir.TestPredicate, bool) {
	if left.Kind == semanticir.ExprCall && right.Kind == semanticir.ExprCall {
		leftRef, okLeft := l.behaviorRef(left)
		rightRef, okRight := l.behaviorRef(right)
		if !okLeft || !okRight {
			return semanticir.TestPredicate{}, false
		}
		return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeEqual, Left: &leftRef, Right: &rightRef, Provenance: provenance}, true
	}
	if left.Kind == semanticir.ExprCall && right.Kind == semanticir.ExprLiteral && right.Literal != nil {
		return l.callOutcomePredicate(left, *right.Literal, provenance)
	}
	if right.Kind == semanticir.ExprCall && left.Kind == semanticir.ExprLiteral && left.Literal != nil {
		return l.callOutcomePredicate(right, *left.Literal, provenance)
	}
	return semanticir.TestPredicate{}, false
}

func (l *lowerer) callOutcomePredicate(call semanticir.Expression, expected semanticir.Literal, provenance semanticir.Provenance) (semanticir.TestPredicate, bool) {
	behavior, ok := l.behaviorRef(call)
	if !ok {
		return semanticir.TestPredicate{}, false
	}
	outcome := semanticir.ObservableOutcome{Kind: semanticir.OutcomeReturn, Value: &expected, OperationID: behavior.OperationID}
	observation := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: behavior, OutcomeIDs: []string{semanticir.OutcomeID(outcome)}, Provenance: provenance}
	return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observation, Provenance: provenance}, true
}

func (l *lowerer) behaviorRef(call semanticir.Expression) (semanticir.BehaviorRef, bool) {
	if call.Kind != semanticir.ExprCall {
		return semanticir.BehaviorRef{}, false
	}
	var declared *semanticir.Operation
	for index := range l.request.Operations {
		operation := &l.request.Operations[index]
		if operation.ID == call.Name || shortName(operation.ID) == shortName(call.Name) {
			if declared != nil && declared.ID != operation.ID {
				return semanticir.BehaviorRef{}, false
			}
			declared = operation
		}
	}
	if declared == nil || len(declared.DomainIDs) != len(call.Operands) {
		return semanticir.BehaviorRef{}, false
	}
	operationID := declared.ID
	domainIDs := append([]string(nil), declared.DomainIDs...)
	literals := make([]semanticir.Literal, len(call.Operands))
	for index, argument := range call.Operands {
		if argument.Kind != semanticir.ExprLiteral || argument.Literal == nil {
			return semanticir.BehaviorRef{}, false
		}
		literals[index] = *argument.Literal
	}
	if exact, inputs, ok := l.exactBehaviorAssignment(*declared, literals); ok {
		return semanticir.BehaviorRef{OperationID: operationID, Conditions: exact, Inputs: inputs, Provenance: call.Provenance}, true
	}
	// Legacy unit fixtures predate typed membership groundings. Preserve their
	// one-literal-per-label behavior only when the entire selected vocabulary
	// has no grounding axioms; strict compiled specs never enter this branch.
	for _, domainID := range domainIDs {
		domain := l.findDomain(domainID)
		if domain == nil {
			return semanticir.BehaviorRef{}, false
		}
		for _, value := range domain.Values {
			if len(value.Groundings) != 0 {
				return semanticir.BehaviorRef{}, false
			}
		}
	}
	conditions := make(semanticir.Assignment, len(call.Operands))
	inputs := make(map[string]semanticir.Literal, len(call.Operands))
	for index, literal := range literals {
		valueID, ok := l.domainValueID(domainIDs[index], literal)
		if !ok {
			return semanticir.BehaviorRef{}, false
		}
		conditions[domainIDs[index]] = valueID
		inputName := domainIDs[index]
		if index < len(declared.Inputs) && declared.Inputs[index].Name != "" {
			inputName = declared.Inputs[index].Name
		}
		inputs[inputName] = literal
	}
	return semanticir.BehaviorRef{OperationID: operationID, Conditions: conditions, Inputs: inputs, Provenance: call.Provenance}, true
}

// exactBehaviorAssignment admits a concrete test call as a category-level
// BehaviorRef only when the selected grounding conjunction uniquely fixes
// every operation input to exactly those call literals. A membership such as
// x>=0 therefore cannot let one test at x=0 speak for the whole category.
func (l *lowerer) exactBehaviorAssignment(operation semanticir.Operation, literals []semanticir.Literal) (semanticir.Assignment, map[string]semanticir.Literal, bool) {
	if len(operation.Inputs) != len(literals) {
		return nil, nil, false
	}
	domains := make([]semanticir.Domain, 0, len(operation.DomainIDs))
	for _, domainID := range operation.DomainIDs {
		domain := l.findDomain(domainID)
		if domain == nil {
			return nil, nil, false
		}
		domains = append(domains, *domain)
	}
	var matched semanticir.Assignment
	var matchedInputs map[string]semanticir.Literal
	for _, assignment := range semanticir.EnumerateAssignments(domains) {
		if l.excludedBehavior(operation.ID, assignment) {
			continue
		}
		bindings, exact := semanticir.ExactGroundingInputs(operation, l.request.FiniteDomains, assignment)
		if !exact {
			continue
		}
		matches := true
		for index, input := range operation.Inputs {
			if !reflect.DeepEqual(bindings[input.Name], literals[index]) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		if matched != nil {
			return nil, nil, false
		}
		matched = cloneAssignment(assignment)
		matchedInputs = cloneLiteralMap(bindings)
	}
	return matched, matchedInputs, matched != nil
}

func (l *lowerer) excludedBehavior(operationID string, assignment semanticir.Assignment) bool {
	for _, constraint := range l.request.Constraints {
		if constraint.OperationID == operationID && reflect.DeepEqual(constraint.Conditions, assignment) {
			return true
		}
	}
	return false
}

func (l *lowerer) domainValueID(domainID string, literal semanticir.Literal) (string, bool) {
	domain := l.findDomain(domainID)
	if domain == nil {
		return "", false
	}
	for _, value := range domain.Values {
		typed, ok := value.TypedValue(*domain)
		if !ok || typed.Type != literal.Type {
			continue
		}
		equal, err := runtimeEqual(runtimeFromLiteral(typed), runtimeFromLiteral(literal))
		if err == nil && equal {
			return value.ID, true
		}
	}
	return "", false
}

func literalFromExpression(expression semanticir.Expression) (semanticir.Literal, bool) {
	if expression.Kind != semanticir.ExprLiteral || expression.Literal == nil {
		return semanticir.Literal{}, false
	}
	return *expression.Literal, true
}

func renderLiteral(literal semanticir.Literal) ([]byte, bool) {
	switch literal.Type {
	case semanticir.TypeBool:
		return []byte(strconv.FormatBool(literal.Bool)), true
	case semanticir.TypeInteger:
		return []byte(strconv.FormatInt(literal.Integer, 10)), true
	case semanticir.TypeString:
		return []byte(strconv.Quote(literal.String)), true
	default:
		return nil, false
	}
}
