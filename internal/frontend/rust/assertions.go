package rust

import (
	"fmt"
	"reflect"

	"github.com/HyperMarble/ray/internal/semanticir"
)

func (l *lowerer) lowerAssertions(fn functionDecl, initial map[string]semanticir.Expression) ([]semanticir.Assertion, string, semanticir.Assignment, semanticir.TestPredicate) {
	bindingsAST := make(map[string]expression)
	bindingsIR := cloneBindings(initial)
	var assertions []semanticir.Assertion
	var predicates []semanticir.TestPredicate
	var refs []semanticir.BehaviorRef

	for _, stmt := range fn.Body.Statements {
		switch stmt.Kind {
		case statementLet:
			bindingsAST[stmt.Name] = substituteAST(stmt.Expr, bindingsAST)
			if lowered, ok := l.lowerExpression(stmt.Expr, bindingsIR); ok {
				bindingsIR[stmt.Name] = lowered
			}
		case statementExpr:
			expr := substituteAST(stmt.Expr, bindingsAST)
			if expr.Kind != expressionMacro || !isAssertionMacro(expr.Text) {
				continue
			}
			assertion, predicate, behaviorRefs, ok := l.lowerAssertion(expr, bindingsIR)
			if ok {
				assertions = append(assertions, assertion)
				predicates = append(predicates, predicate)
				refs = append(refs, behaviorRefs...)
			}
		case statementReturn:
			l.block(stmt.Span, semanticir.DiagnosticUnsupported, "return in a Rust test is not an assertion")
		}
	}
	if fn.Body.Tail != nil {
		l.block(fn.Body.Tail.Span, semanticir.DiagnosticUnsupported, "tail expression in a Rust test is not an assertion")
	}

	operationID := ""
	conditions := semanticir.Assignment{}
	if len(refs) > 0 {
		operationID = refs[0].OperationID
		conditions = cloneAssignment(refs[0].Conditions)
		for _, ref := range refs[1:] {
			if ref.OperationID != operationID || !assignmentsEqual(ref.Conditions, conditions) {
				operationID = ""
				conditions = semanticir.Assignment{}
				break
			}
		}
	}
	predicate := semanticir.TestPredicate{Kind: semanticir.PredicateTrue, Provenance: l.prov(fn.Span)}
	if len(predicates) == 1 {
		predicate = predicates[0]
	} else if len(predicates) > 1 {
		predicate = semanticir.TestPredicate{Kind: semanticir.PredicateAnd, Children: predicates, Provenance: l.prov(fn.Span)}
	}
	return assertions, operationID, conditions, predicate
}

func (l *lowerer) lowerAssertion(expr expression, bindings map[string]semanticir.Expression) (semanticir.Assertion, semanticir.TestPredicate, []semanticir.BehaviorRef, bool) {
	prov := l.prov(expr.Span)
	message, messageOK := assertionMessage(expr)
	if !messageOK {
		l.block(expr.Span, semanticir.DiagnosticUnsupported, expr.Text+"! message must be a static string literal")
		return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
	}
	switch expr.Text {
	case "assert_eq", "assert_ne":
		if len(expr.Children) < 2 || len(expr.Children) > 3 {
			l.block(expr.Span, semanticir.DiagnosticInvalidInput, expr.Text+"! requires two values and an optional static message")
			return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
		}
		actual, actualOK := l.lowerExpression(expr.Children[0], bindings)
		expected, expectedOK := l.lowerExpression(expr.Children[1], bindings)
		if !actualOK || !expectedOK {
			return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
		}
		kind := semanticir.AssertEqual
		if expr.Text == "assert_ne" {
			kind = semanticir.AssertNotEqual
		}
		assertion := semanticir.Assertion{Kind: kind, Actual: &actual, Expected: &expected, Message: message, Provenance: prov}

		leftRef, leftCall := l.behaviorRef(expr.Children[0])
		rightRef, rightCall := l.behaviorRef(expr.Children[1])
		if leftCall && rightCall {
			predicate := semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeEqual, Left: &leftRef, Right: &rightRef, Provenance: prov}
			if kind == semanticir.AssertNotEqual {
				predicate = semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{predicate}, Provenance: prov}
			}
			return assertion, predicate, []semanticir.BehaviorRef{leftRef, rightRef}, true
		}
		if leftCall == rightCall {
			l.block(expr.Span, semanticir.DiagnosticUnsupported, expr.Text+"! must compare one behavior call with a bounded literal/result, or two behavior calls")
			return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
		}
		behavior := leftRef
		expectedExpr := expr.Children[1]
		if rightCall {
			behavior = rightRef
			expectedExpr = expr.Children[0]
		}
		outcomeID, ok := expectedOutcomeID(behavior.OperationID, expectedExpr)
		if !ok {
			l.block(expectedExpr.Span, semanticir.DiagnosticUnsupported, "asserted Rust value cannot be rendered as a finite observable outcome")
			return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
		}
		assertion.OutcomeIDs = []string{outcomeID}
		observation := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: behavior, OutcomeIDs: []string{outcomeID}, Provenance: prov}
		predicate := semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observation, Provenance: prov}
		if kind == semanticir.AssertNotEqual {
			predicate = semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{predicate}, Provenance: prov}
		}
		return assertion, predicate, []semanticir.BehaviorRef{behavior}, true

	case "assert":
		if len(expr.Children) < 1 || len(expr.Children) > 2 {
			l.block(expr.Span, semanticir.DiagnosticInvalidInput, "assert! requires a condition and optional static message")
			return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
		}
		actual, ok := l.lowerExpression(expr.Children[0], bindings)
		if !ok {
			return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
		}
		assertion := semanticir.Assertion{Kind: semanticir.AssertTrue, Actual: &actual, Message: message, Provenance: prov}
		predicate, behaviorRefs, ok := l.booleanTestPredicate(expr.Children[0], true)
		if !ok {
			l.block(expr.Children[0].Span, semanticir.DiagnosticUnsupported, "assert! condition is not an exact predicate over bounded behavior calls")
			return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
		}
		return assertion, predicate, behaviorRefs, true
	}
	l.block(expr.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("unresolved macro %s!", expr.Text))
	return semanticir.Assertion{}, semanticir.TestPredicate{}, nil, false
}

func (l *lowerer) booleanTestPredicate(expr expression, expected bool) (semanticir.TestPredicate, []semanticir.BehaviorRef, bool) {
	prov := l.prov(expr.Span)
	if expr.Kind == expressionUnary && expr.Text == "!" && len(expr.Children) == 1 {
		predicate, refs, ok := l.booleanTestPredicate(expr.Children[0], !expected)
		return predicate, refs, ok
	}
	if expr.Kind == expressionBinary && (expr.Text == "&&" || expr.Text == "||") && expected {
		left, leftRefs, leftOK := l.booleanTestPredicate(expr.Children[0], true)
		right, rightRefs, rightOK := l.booleanTestPredicate(expr.Children[1], true)
		if !leftOK || !rightOK {
			return semanticir.TestPredicate{}, nil, false
		}
		kind := semanticir.PredicateAnd
		if expr.Text == "||" {
			kind = semanticir.PredicateOr
		}
		return semanticir.TestPredicate{Kind: kind, Children: []semanticir.TestPredicate{left, right}, Provenance: prov}, append(leftRefs, rightRefs...), true
	}
	if expr.Kind == expressionBinary && (expr.Text == "==" || expr.Text == "!=") {
		left, leftOK := l.behaviorRef(expr.Children[0])
		right, rightOK := l.behaviorRef(expr.Children[1])
		if !leftOK || !rightOK {
			return semanticir.TestPredicate{}, nil, false
		}
		predicate := semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeEqual, Left: &left, Right: &right, Provenance: prov}
		equalityExpected := expected == (expr.Text == "==")
		if !equalityExpected {
			predicate = semanticir.TestPredicate{Kind: semanticir.PredicateNot, Children: []semanticir.TestPredicate{predicate}, Provenance: prov}
		}
		return predicate, []semanticir.BehaviorRef{left, right}, true
	}
	behavior, ok := l.behaviorRef(expr)
	if !ok {
		return semanticir.TestPredicate{}, nil, false
	}
	literal := expression{Kind: expressionIdentifier, Text: fmt.Sprintf("%t", expected), Span: expr.Span}
	outcomeID, _ := expectedOutcomeID(behavior.OperationID, literal)
	observation := semanticir.Observation{Kind: semanticir.ObserveOutcome, Behavior: behavior, OutcomeIDs: []string{outcomeID}, Provenance: prov}
	return semanticir.TestPredicate{Kind: semanticir.PredicateOutcomeIn, Observe: &observation, Provenance: prov}, []semanticir.BehaviorRef{behavior}, true
}

func (l *lowerer) behaviorRef(expr expression) (semanticir.BehaviorRef, bool) {
	if expr.Kind != expressionCall || expr.Text == "Ok" || expr.Text == "Err" || expr.Text == "Result::Ok" || expr.Text == "Result::Err" {
		return semanticir.BehaviorRef{}, false
	}
	if _, local := l.functions[expr.Text]; !local && !containsString(l.request.EntryPoints, expr.Text) {
		return semanticir.BehaviorRef{}, false
	}
	condition := make(semanticir.Assignment)
	inputs := make(map[string]semanticir.Literal, len(expr.Children))
	parameterDomains := l.callDomainIDs(expr.Text, len(expr.Children))
	operation, declared := requestOperation(l.request.Operations, expr.Text)
	if len(parameterDomains) != len(expr.Children) || !declared || len(operation.Inputs) != len(expr.Children) {
		l.block(expr.Span, semanticir.DiagnosticMissingDomain, fmt.Sprintf("call %s has %d arguments but finite domains do not identify all parameters", expr.Text, len(expr.Children)))
		return semanticir.BehaviorRef{}, false
	}
	for index, arg := range expr.Children {
		literal, ok := constantExpressionLiteral(arg)
		if !ok {
			l.block(arg.Span, semanticir.DiagnosticUnsupported, "behavior-call arguments in tests must be bounded scalar literals")
			return semanticir.BehaviorRef{}, false
		}
		domain, ok := findDomain(l.request.FiniteDomains, parameterDomains[index])
		if !ok {
			return semanticir.BehaviorRef{}, false
		}
		valueID, ok := findDomainValueID(domain, literal, expr.Text, operation.Inputs[index].Name)
		if !ok {
			l.block(arg.Span, semanticir.DiagnosticInvalidReference, fmt.Sprintf("call argument is not a member of finite domain %s", domain.ID))
			return semanticir.BehaviorRef{}, false
		}
		condition[domain.ID] = valueID
		inputs[operation.Inputs[index].Name] = literal
	}
	return semanticir.BehaviorRef{OperationID: expr.Text, Conditions: condition, Inputs: inputs, Provenance: l.prov(expr.Span)}, true
}

func (l *lowerer) callDomainIDs(operation string, arity int) []string {
	if fn, ok := l.functions[operation]; ok {
		result := make([]string, 0, len(fn.Parameters))
		for _, param := range fn.Parameters {
			result = append(result, findDomainID(l.request, operation, param.Name))
		}
		return result
	}
	declared, ok := requestOperation(l.request.Operations, operation)
	if !ok || len(declared.DomainIDs) != arity {
		return nil
	}
	return append([]string(nil), declared.DomainIDs...)
}

func expectedOutcomeID(operation string, expr expression) (string, bool) {
	value, ok := observableFromExpected(expr)
	if !ok {
		return "", false
	}
	return outcomeID(operation, value), true
}

func observableFromExpected(expr expression) (evaluatedOutcome, bool) {
	if expr.Kind == expressionCall && (expr.Text == "Ok" || expr.Text == "Result::Ok" || expr.Text == "Err" || expr.Text == "Result::Err") && len(expr.Children) == 1 {
		literal, ok := constantExpressionLiteral(expr.Children[0])
		if !ok {
			return evaluatedOutcome{}, false
		}
		kind := semanticir.OutcomeSuccess
		exception := ""
		if expr.Text == "Err" || expr.Text == "Result::Err" {
			kind = semanticir.OutcomeRaise
			exception = "Result::Err"
		}
		_ = literal // rustc validates the payload; shared Result outcomes are variant-only.
		return evaluatedOutcome{Kind: kind, ExceptionType: exception}, true
	}
	literal, ok := constantExpressionLiteral(expr)
	if !ok {
		return evaluatedOutcome{}, false
	}
	return evaluatedOutcome{Kind: semanticir.OutcomeReturn, Literal: &literal}, true
}

func constantExpressionLiteral(expr expression) (semanticir.Literal, bool) {
	if expr.Kind == expressionIdentifier && (expr.Text == "true" || expr.Text == "false") {
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: expr.Text == "true"}, true
	}
	if expr.Kind == expressionLiteral {
		return parseRustLiteral(expr.Text)
	}
	if expr.Kind == expressionTuple && expr.Text == "()" && len(expr.Children) == 0 {
		return semanticir.Literal{Type: semanticir.TypeUnit}, true
	}
	if expr.Kind == expressionUnary && expr.Text == "-" && len(expr.Children) == 1 {
		literal, ok := constantExpressionLiteral(expr.Children[0])
		if !ok || literal.Type != semanticir.TypeInteger || literal.Integer == -literal.Integer {
			return semanticir.Literal{}, false
		}
		literal.Integer = -literal.Integer
		return literal, true
	}
	return semanticir.Literal{}, false
}

func assertionMessage(expr expression) (string, bool) {
	messageIndex := -1
	if expr.Text == "assert" && len(expr.Children) == 2 {
		messageIndex = 1
	}
	if (expr.Text == "assert_eq" || expr.Text == "assert_ne") && len(expr.Children) == 3 {
		messageIndex = 2
	}
	if messageIndex < 0 {
		return "", true
	}
	literal, ok := constantExpressionLiteral(expr.Children[messageIndex])
	return literal.String, ok && literal.Type == semanticir.TypeString
}

func substituteAST(expr expression, bindings map[string]expression) expression {
	if expr.Kind == expressionIdentifier {
		if value, ok := bindings[expr.Text]; ok {
			value.Span = expr.Span
			return value
		}
	}
	copyExpr := expr
	copyExpr.Children = make([]expression, len(expr.Children))
	for index, child := range expr.Children {
		copyExpr.Children[index] = substituteAST(child, bindings)
	}
	return copyExpr
}

func findDomain(domains []semanticir.Domain, id string) (semanticir.Domain, bool) {
	for _, domain := range domains {
		if domain.ID == id {
			return domain, true
		}
	}
	return semanticir.Domain{}, false
}

func findDomainValueID(domain semanticir.Domain, literal semanticir.Literal, operationID, inputName string) (string, bool) {
	match := ""
	for _, value := range domain.Values {
		parsed, ok := value.TypedValue(domain)
		if len(value.Groundings) != 0 {
			parsed, ok = rustLiteralForDomainValue(domain, value, operationID, inputName, literal.Type)
		}
		if (!ok || parsed.Type != literal.Type) && (domain.Type == semanticir.TypeUnknown || domain.Type == "") && value.Value == nil {
			parsed, ok = domainLiteral(value.ID, literal.Type)
		}
		if ok && reflect.DeepEqual(parsed, literal) {
			if match != "" {
				return "", false
			}
			match = value.ID
		}
	}
	return match, match != ""
}

func domainLiteral(id string, valueType semanticir.ValueType) (semanticir.Literal, bool) {
	switch valueType {
	case semanticir.TypeString:
		if parsed, ok := parseRustLiteral(id); ok && parsed.Type == semanticir.TypeString {
			return parsed, true
		}
		return semanticir.Literal{Type: semanticir.TypeString, String: id}, true
	case semanticir.TypeBool:
		if id == "true" || id == "false" {
			return semanticir.Literal{Type: semanticir.TypeBool, Bool: id == "true"}, true
		}
	case semanticir.TypeInteger:
		return parseRustLiteral(id)
	case semanticir.TypeUnit:
		return semanticir.Literal{Type: semanticir.TypeUnit}, id == "()"
	}
	return semanticir.Literal{}, false
}

func assignmentsEqual(left, right semanticir.Assignment) bool {
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

func cloneAssignment(source semanticir.Assignment) semanticir.Assignment {
	result := make(semanticir.Assignment, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
