package semanticir

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

// GroundingFor returns the single frozen grounding axiom for one semantic
// label in one operation. The boolean is false when the label is not grounded
// exactly once; callers must treat that as proof-blocking rather than choosing
// an arbitrary axiom.
func (value DomainValue) GroundingFor(operationID string) (GroundingAxiom, bool) {
	var result GroundingAxiom
	found := false
	for _, grounding := range value.Groundings {
		if grounding.OperationID != operationID {
			continue
		}
		if found {
			return GroundingAxiom{}, false
		}
		result = grounding
		found = true
	}
	return result, found
}

// GroundingConjunction returns the closed membership formula for one exact
// semantic assignment. The conjunction contains no required outcomes.
func GroundingConjunction(operation Operation, domains []Domain, assignment Assignment, provenance Provenance) (Expression, error) {
	if len(assignment) != len(operation.DomainIDs) {
		return Expression{}, fmt.Errorf("assignment does not cover exactly operation %q domains", operation.ID)
	}
	var predicates []Expression
	for _, domainID := range operation.DomainIDs {
		domain, exists := domainByID(domains, domainID)
		if !exists {
			return Expression{}, fmt.Errorf("operation %q refers to unknown domain %q", operation.ID, domainID)
		}
		valueID, exists := assignment[domainID]
		if !exists {
			return Expression{}, fmt.Errorf("assignment omits domain %q", domainID)
		}
		var value *DomainValue
		for index := range domain.Values {
			if domain.Values[index].ID == valueID {
				value = &domain.Values[index]
				break
			}
		}
		if value == nil {
			return Expression{}, fmt.Errorf("assignment selects unknown domain %q value %q", domainID, valueID)
		}
		grounding, ok := value.GroundingFor(operation.ID)
		if !ok || grounding.Kind != GroundingMembership || grounding.Membership == nil {
			return Expression{}, fmt.Errorf("operation %q domain %q value %q is not uniquely grounded", operation.ID, domainID, valueID)
		}
		predicates = append(predicates, *grounding.Membership)
	}
	if len(predicates) == 0 {
		literal := Literal{Type: TypeBool, Bool: true}
		return Expression{Kind: ExprLiteral, Type: TypeBool, Literal: &literal, Provenance: provenance}, nil
	}
	result := predicates[0]
	for _, predicate := range predicates[1:] {
		result = Expression{Kind: ExprBool, Type: TypeBool, Operator: OpAnd, Operands: []Expression{result, predicate}, Provenance: provenance}
	}
	return result, nil
}

// ExactGroundingInputs reports whether the complete selected-label
// conjunction uniquely fixes every operation input to a typed literal. A
// category witness alone never makes this true.
func ExactGroundingInputs(operation Operation, domains []Domain, assignment Assignment) (map[string]Literal, bool) {
	conjunction, err := GroundingConjunction(operation, domains, assignment, operation.Provenance)
	if err != nil {
		return nil, false
	}
	bindings := map[string]Literal{}
	if len(operation.Inputs) == 0 {
		return bindings, len(operation.DomainIDs) == 0
	}
	if !collectExactGroundingBindings(conjunction, bindings) {
		return nil, false
	}
	for _, input := range operation.Inputs {
		literal, exists := bindings[input.Name]
		if !exists || literal.Type != input.Type || ValidateLiteral(literal) != nil {
			return nil, false
		}
	}
	if len(bindings) != len(operation.Inputs) {
		return nil, false
	}
	return bindings, true
}

// AssignmentGroundingID is the deterministic identifier shared by the
// outcome-free grounding registry and RequirementCase references.
func AssignmentGroundingID(operationID string, assignment Assignment) string {
	digest, _ := Digest(struct {
		OperationID string     `json:"operation_id"`
		Conditions  Assignment `json:"conditions"`
	}{operationID, assignment})
	return "grounding-" + strings.TrimPrefix(digest, "sha256:")[:20]
}

func collectExactGroundingBindings(expression Expression, bindings map[string]Literal) bool {
	if expression.Kind == ExprBool && expression.Operator == OpAnd && len(expression.Operands) == 2 {
		return collectExactGroundingBindings(expression.Operands[0], bindings) && collectExactGroundingBindings(expression.Operands[1], bindings)
	}
	if expression.Kind != ExprCompare || expression.Operator != OpEQ || len(expression.Operands) != 2 {
		return false
	}
	left, right := expression.Operands[0], expression.Operands[1]
	if left.Kind == ExprLiteral && right.Kind == ExprVariable {
		left, right = right, left
	}
	if left.Kind != ExprVariable || right.Kind != ExprLiteral || right.Literal == nil || left.Type != right.Literal.Type {
		return false
	}
	if previous, exists := bindings[left.Name]; exists && !reflect.DeepEqual(previous, *right.Literal) {
		return false
	}
	bindings[left.Name] = *right.Literal
	return true
}

// EvaluateGroundingMembership evaluates the closed author-visible grounding
// vocabulary against a complete concrete witness. It supports checked
// mathematical integer +,-,* and boolean/comparison expressions, but no
// calls, field/index access, or source-language behavior.
func EvaluateGroundingMembership(expression Expression, witness map[string]Literal) (bool, error) {
	value, err := evaluateGroundingExpression(expression, witness)
	if err != nil {
		return false, err
	}
	if value.Type != TypeBool {
		return false, fmt.Errorf("grounding expression evaluates to %q, want bool", value.Type)
	}
	return value.Bool, nil
}

func evaluateGroundingExpression(expression Expression, witness map[string]Literal) (Literal, error) {
	switch expression.Kind {
	case ExprLiteral:
		if expression.Literal == nil {
			return Literal{}, fmt.Errorf("literal expression has no literal")
		}
		return *expression.Literal, nil
	case ExprVariable:
		value, exists := witness[expression.Name]
		if !exists {
			return Literal{}, fmt.Errorf("witness omits input %q", expression.Name)
		}
		return value, nil
	case ExprUnary:
		if len(expression.Operands) != 1 {
			return Literal{}, fmt.Errorf("grounding unary expression has invalid arity")
		}
		value, err := evaluateGroundingExpression(expression.Operands[0], witness)
		if err != nil {
			return Literal{}, err
		}
		switch expression.Operator {
		case OpNot:
			if value.Type != TypeBool {
				return Literal{}, fmt.Errorf("grounding not operand is not boolean")
			}
			return Literal{Type: TypeBool, Bool: !value.Bool}, nil
		case OpNeg:
			if value.Type != TypeInteger {
				return Literal{}, fmt.Errorf("grounding negation operand is not integer")
			}
			integer := new(big.Int).Neg(big.NewInt(value.Integer))
			if !integer.IsInt64() {
				return Literal{}, fmt.Errorf("grounding integer negation overflows signed 64-bit")
			}
			return Literal{Type: TypeInteger, Integer: integer.Int64()}, nil
		default:
			return Literal{}, fmt.Errorf("grounding unary expression uses unsupported operator %q", expression.Operator)
		}
	case ExprBinary:
		if len(expression.Operands) != 2 || (expression.Operator != OpAdd && expression.Operator != OpSub && expression.Operator != OpMul) {
			return Literal{}, fmt.Errorf("grounding arithmetic expression has unsupported operator or arity")
		}
		left, err := evaluateGroundingExpression(expression.Operands[0], witness)
		if err != nil {
			return Literal{}, err
		}
		right, err := evaluateGroundingExpression(expression.Operands[1], witness)
		if err != nil {
			return Literal{}, err
		}
		if left.Type != TypeInteger || right.Type != TypeInteger {
			return Literal{}, fmt.Errorf("grounding arithmetic operands are not integers")
		}
		integer := new(big.Int)
		switch expression.Operator {
		case OpAdd:
			integer.Add(big.NewInt(left.Integer), big.NewInt(right.Integer))
		case OpSub:
			integer.Sub(big.NewInt(left.Integer), big.NewInt(right.Integer))
		case OpMul:
			integer.Mul(big.NewInt(left.Integer), big.NewInt(right.Integer))
		}
		if !integer.IsInt64() {
			return Literal{}, fmt.Errorf("grounding integer arithmetic overflows signed 64-bit")
		}
		return Literal{Type: TypeInteger, Integer: integer.Int64()}, nil
	case ExprBool:
		if len(expression.Operands) != 2 || (expression.Operator != OpAnd && expression.Operator != OpOr) {
			return Literal{}, fmt.Errorf("grounding boolean expression has invalid operator or arity")
		}
		left, err := evaluateGroundingExpression(expression.Operands[0], witness)
		if err != nil {
			return Literal{}, err
		}
		right, err := evaluateGroundingExpression(expression.Operands[1], witness)
		if err != nil {
			return Literal{}, err
		}
		if left.Type != TypeBool || right.Type != TypeBool {
			return Literal{}, fmt.Errorf("grounding boolean operands are not boolean")
		}
		if expression.Operator == OpAnd {
			return Literal{Type: TypeBool, Bool: left.Bool && right.Bool}, nil
		}
		return Literal{Type: TypeBool, Bool: left.Bool || right.Bool}, nil
	case ExprCompare:
		if len(expression.Operands) != 2 {
			return Literal{}, fmt.Errorf("grounding comparison has invalid arity")
		}
		left, err := evaluateGroundingExpression(expression.Operands[0], witness)
		if err != nil {
			return Literal{}, err
		}
		right, err := evaluateGroundingExpression(expression.Operands[1], witness)
		if err != nil {
			return Literal{}, err
		}
		if left.Type != right.Type {
			return Literal{}, fmt.Errorf("grounding comparison mixes %q and %q", left.Type, right.Type)
		}
		var result bool
		switch expression.Operator {
		case OpEQ:
			result = reflect.DeepEqual(left, right)
		case OpNE:
			result = !reflect.DeepEqual(left, right)
		case OpLT, OpLE, OpGT, OpGE:
			if left.Type != TypeInteger {
				return Literal{}, fmt.Errorf("ordered grounding comparison requires integer operands")
			}
			switch expression.Operator {
			case OpLT:
				result = left.Integer < right.Integer
			case OpLE:
				result = left.Integer <= right.Integer
			case OpGT:
				result = left.Integer > right.Integer
			case OpGE:
				result = left.Integer >= right.Integer
			}
		default:
			return Literal{}, fmt.Errorf("grounding comparison uses unsupported operator %q", expression.Operator)
		}
		return Literal{Type: TypeBool, Bool: result}, nil
	default:
		return Literal{}, fmt.Errorf("grounding uses unsupported expression kind %q", expression.Kind)
	}
}

func validateGroundingAxioms(task *Task, operations map[string]Operation) []Diagnostic {
	var diagnostics []Diagnostic
	type groundingKey struct {
		operation string
		domain    string
		value     string
	}
	expected := map[groundingKey]Provenance{}
	for _, operation := range operations {
		for _, domainID := range operation.DomainIDs {
			domain, exists := domainByID(task.Domains, domainID)
			if !exists {
				continue
			}
			for _, value := range domain.Values {
				expected[groundingKey{operation.ID, domainID, value.ID}] = value.Provenance
			}
		}
	}

	seen := map[groundingKey]struct{}{}
	for _, domain := range task.Domains {
		for _, value := range domain.Values {
			for _, grounding := range value.Groundings {
				key := groundingKey{grounding.OperationID, domain.ID, value.ID}
				operation, operationExists := operations[grounding.OperationID]
				if !operationExists || !containsString(operation.DomainIDs, domain.ID) {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("domain %q value %q grounding refers to unrelated operation %q", domain.ID, value.ID, grounding.OperationID), grounding.Provenance))
					continue
				}
				if _, duplicate := seen[key]; duplicate {
					diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("operation %q domain %q value %q has duplicate groundings", grounding.OperationID, domain.ID, value.ID), grounding.Provenance))
					continue
				}
				seen[key] = struct{}{}
				diagnostics = append(diagnostics, validateGroundingAxiom(task.Spec, operation, domain, value, grounding)...)
			}
		}
	}
	for key, provenance := range expected {
		if _, exists := seen[key]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("operation %q domain %q value %q has no frozen grounding axiom", key.operation, key.domain, key.value), provenance))
		}
	}
	return diagnostics
}

func validateAssignmentGroundings(task *Task, operations map[string]Operation, domainValues map[string]map[string]struct{}) ([]Diagnostic, map[string]AssignmentGrounding, map[string]AssignmentGrounding) {
	var diagnostics []Diagnostic
	byID := map[string]AssignmentGrounding{}
	byBehavior := map[string]AssignmentGrounding{}
	for _, grounding := range task.Groundings {
		if err := validateFactSource(grounding.Provenance, task.Spec); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "assignment grounding: "+err.Error(), grounding.Provenance))
		}
		operation, exists := operations[grounding.OperationID]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("assignment grounding %q refers to unknown operation %q", grounding.ID, grounding.OperationID), grounding.Provenance))
			continue
		}
		if err := validateAssignment(grounding.Conditions, operationDomainValues(operation, domainValues)); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("assignment grounding %q: %v", grounding.ID, err), grounding.Provenance))
			continue
		}
		wantID := AssignmentGroundingID(grounding.OperationID, grounding.Conditions)
		if grounding.ID != wantID {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("assignment grounding %q canonical ID is %q", grounding.ID, wantID), grounding.Provenance))
		}
		if _, duplicate := byID[grounding.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("duplicate assignment grounding ID %q", grounding.ID), grounding.Provenance))
		}
		behavior := behaviorKey(grounding.OperationID, grounding.Conditions)
		if previous, duplicate := byBehavior[behavior]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, fmt.Sprintf("semantic behavior %s has multiple concrete witnesses (%q and %q)", behavior, previous.ID, grounding.ID), grounding.Provenance))
		}
		byID[grounding.ID] = grounding
		byBehavior[behavior] = grounding

		inputs := inputsByName(operation.Inputs)
		if len(grounding.Inputs) != len(inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("assignment grounding %q does not assign every operation input", grounding.ID), grounding.Provenance))
		}
		for name, literal := range grounding.Inputs {
			input, exists := inputs[name]
			if !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("assignment grounding %q assigns unknown input %q", grounding.ID, name), grounding.Provenance))
				continue
			}
			if err := ValidateLiteral(literal); err != nil || literal.Type != input.Type {
				if err == nil {
					err = fmt.Errorf("type %q differs from input type %q", literal.Type, input.Type)
				}
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("assignment grounding %q input %q: %v", grounding.ID, name, err), grounding.Provenance))
			}
			if len(input.Universe) != 0 && !literalInUniverse(literal, input.Universe) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("assignment grounding %q input %q lies outside its frozen Universe", grounding.ID, name), grounding.Provenance))
			}
		}
		conjunction, err := GroundingConjunction(operation, task.Domains, grounding.Conditions, grounding.Provenance)
		if err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("assignment grounding %q: %v", grounding.ID, err), grounding.Provenance))
		} else if satisfied, evaluationErr := EvaluateGroundingMembership(conjunction, grounding.Inputs); evaluationErr != nil || !satisfied {
			if evaluationErr == nil {
				evaluationErr = fmt.Errorf("witness does not satisfy selected label conjunction")
			}
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, fmt.Sprintf("assignment grounding %q: %v", grounding.ID, evaluationErr), grounding.Provenance))
		}
	}
	return diagnostics, byID, byBehavior
}

func literalInUniverse(literal Literal, universe []Literal) bool {
	for _, candidate := range universe {
		if reflect.DeepEqual(literal, candidate) {
			return true
		}
	}
	return false
}

func validateGroundingAxiom(spec ArtifactRef, operation Operation, domain Domain, value DomainValue, grounding GroundingAxiom) []Diagnostic {
	var diagnostics []Diagnostic
	label := fmt.Sprintf("operation %q domain %q value %q grounding", operation.ID, domain.ID, value.ID)
	if err := validateFactSource(grounding.Provenance, spec); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, label+": "+err.Error(), grounding.Provenance))
	}
	if grounding.Kind != GroundingMembership || grounding.Membership == nil || len(grounding.Exact) != 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" must contain exactly one membership expression", grounding.Provenance))
		return diagnostics
	}
	expression := *grounding.Membership
	diagnostics = append(diagnostics, validateExpression(expression, spec, label+" membership")...)
	if err := validateGroundingExpressionShape(expression, inputsByName(operation.Inputs)); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, label+": "+err.Error(), expression.Provenance))
	}
	if expression.Type != TypeBool {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" membership is not boolean", expression.Provenance))
	}

	inputs := make(map[string]Variable, len(operation.Inputs))
	for _, input := range operation.Inputs {
		inputs[input.Name] = input
	}
	for variable := range expressionVariables(expression) {
		if _, exists := inputs[variable]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("%s references undeclared input %q", label, variable), expression.Provenance))
		}
	}
	if len(grounding.ConcreteWitness) != len(inputs) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, label+" witness does not assign every operation input exactly once", grounding.Provenance))
	}
	for name, literal := range grounding.ConcreteWitness {
		input, exists := inputs[name]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, fmt.Sprintf("%s witness assigns unknown input %q", label, name), grounding.Provenance))
			continue
		}
		if err := ValidateLiteral(literal); err != nil || literal.Type != input.Type {
			if err == nil {
				err = fmt.Errorf("type %q differs from input type %q", literal.Type, input.Type)
			}
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("%s witness input %q: %v", label, name, err), grounding.Provenance))
		}
	}
	if !HasErrors(diagnostics) {
		satisfied, err := EvaluateGroundingMembership(expression, grounding.ConcreteWitness)
		if err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" cannot be evaluated: "+err.Error(), grounding.Provenance))
		} else if !satisfied {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, label+" witness does not satisfy its membership expression", grounding.Provenance))
		}
	}
	return diagnostics
}

func inputsByName(inputs []Variable) map[string]Variable {
	result := make(map[string]Variable, len(inputs))
	for _, input := range inputs {
		result[input.Name] = input
	}
	return result
}

func validateGroundingExpressionShape(expression Expression, inputs map[string]Variable) error {
	switch expression.Kind {
	case ExprLiteral:
		if expression.Literal == nil || expression.Type != expression.Literal.Type || ValidateLiteral(*expression.Literal) != nil {
			return fmt.Errorf("grounding contains an invalid typed literal")
		}
	case ExprVariable:
		input, exists := inputs[expression.Name]
		if !exists || input.Type != expression.Type {
			return fmt.Errorf("grounding variable %q does not match a typed operation input", expression.Name)
		}
	case ExprUnary:
		if len(expression.Operands) != 1 {
			return fmt.Errorf("grounding unary expression has invalid arity")
		}
		operand := expression.Operands[0]
		if (expression.Operator == OpNot && (expression.Type != TypeBool || operand.Type != TypeBool)) || (expression.Operator == OpNeg && (expression.Type != TypeInteger || operand.Type != TypeInteger)) {
			return fmt.Errorf("grounding unary expression has incompatible types")
		}
		if expression.Operator != OpNot && expression.Operator != OpNeg {
			return fmt.Errorf("grounding unary operator %q is unsupported", expression.Operator)
		}
	case ExprBinary:
		if len(expression.Operands) != 2 || expression.Type != TypeInteger || expression.Operands[0].Type != TypeInteger || expression.Operands[1].Type != TypeInteger {
			return fmt.Errorf("grounding arithmetic requires two integer operands and result")
		}
		if expression.Operator != OpAdd && expression.Operator != OpSub && expression.Operator != OpMul {
			return fmt.Errorf("grounding arithmetic operator %q is unsupported", expression.Operator)
		}
	case ExprCompare:
		if len(expression.Operands) != 2 || expression.Type != TypeBool || expression.Operands[0].Type != expression.Operands[1].Type {
			return fmt.Errorf("grounding comparison has incompatible types or arity")
		}
		if expression.Operator != OpEQ && expression.Operator != OpNE && expression.Operands[0].Type != TypeInteger {
			return fmt.Errorf("ordered grounding comparison requires integer operands")
		}
		switch expression.Operator {
		case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE:
		default:
			return fmt.Errorf("grounding comparison operator %q is unsupported", expression.Operator)
		}
	case ExprBool:
		if len(expression.Operands) != 2 || expression.Type != TypeBool || expression.Operands[0].Type != TypeBool || expression.Operands[1].Type != TypeBool || (expression.Operator != OpAnd && expression.Operator != OpOr) {
			return fmt.Errorf("grounding boolean expression has incompatible types, operator, or arity")
		}
	default:
		return fmt.Errorf("grounding expression kind %q is unsupported", expression.Kind)
	}
	for _, operand := range expression.Operands {
		if err := validateGroundingExpressionShape(operand, inputs); err != nil {
			return err
		}
	}
	return nil
}
