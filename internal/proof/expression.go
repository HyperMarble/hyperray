package proof

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func evaluateInvariant(task *semanticir.Task, invariant semanticir.Invariant, assignment semanticir.Assignment, outcome semanticir.ObservableOutcome) (bool, error) {
	environment := make(map[string]semanticir.Literal, len(invariant.Bindings))
	for _, binding := range invariant.Bindings {
		var value semanticir.Literal
		switch binding.Kind {
		case semanticir.BindDomainValue:
			valueID, exists := assignment[binding.DomainID]
			if !exists {
				return false, fmt.Errorf("domain binding %q is outside this operation assignment", binding.DomainID)
			}
			literal, exists := taskDomainLiteral(task, binding.DomainID, valueID)
			if !exists {
				return false, fmt.Errorf("domain binding %q value %q has no typed literal", binding.DomainID, valueID)
			}
			value = literal
		case semanticir.BindOutcomeValue:
			if outcome.Value == nil {
				return false, fmt.Errorf("outcome %q has no value for binding %q", outcome.ID, binding.Variable)
			}
			value = *outcome.Value
		case semanticir.BindEffectValue:
			var matches []semanticir.Effect
			for _, effect := range outcome.Effects {
				if effect.Kind == binding.EffectKind && effect.Target == binding.EffectTarget {
					matches = append(matches, effect)
				}
			}
			if len(matches) != 1 || matches[0].Value == nil {
				return false, fmt.Errorf("effect binding %q needs exactly one valued %s effect on %q; found %d", binding.Variable, binding.EffectKind, binding.EffectTarget, len(matches))
			}
			resolved, err := evaluateExpression(*matches[0].Value, nil)
			if err != nil {
				return false, fmt.Errorf("effect binding %q: %w", binding.Variable, err)
			}
			value = resolved
		default:
			return false, fmt.Errorf("unsupported invariant binding kind %q", binding.Kind)
		}
		environment[binding.Variable] = value
	}
	result, err := evaluateExpression(invariant.Predicate, environment)
	if err != nil {
		return false, err
	}
	if result.Type != semanticir.TypeBool {
		return false, fmt.Errorf("invariant result has type %q, want bool", result.Type)
	}
	return result.Bool, nil
}

func taskDomainLiteral(task *semanticir.Task, domainID, valueID string) (semanticir.Literal, bool) {
	for _, domain := range task.Domains {
		if domain.ID != domainID {
			continue
		}
		for _, value := range domain.Values {
			if value.ID == valueID {
				return value.TypedValue(domain)
			}
		}
	}
	return semanticir.Literal{}, false
}

func evaluateExpression(expression semanticir.Expression, environment map[string]semanticir.Literal) (semanticir.Literal, error) {
	var result semanticir.Literal
	var err error
	switch expression.Kind {
	case semanticir.ExprLiteral:
		if expression.Literal == nil {
			return result, fmt.Errorf("literal expression has no literal")
		}
		result = *expression.Literal
	case semanticir.ExprVariable:
		var exists bool
		result, exists = environment[expression.Name]
		if !exists {
			return result, fmt.Errorf("unbound variable %q", expression.Name)
		}
	case semanticir.ExprUnary:
		if len(expression.Operands) != 1 {
			return result, fmt.Errorf("unary expression has %d operands", len(expression.Operands))
		}
		operand, operandErr := evaluateExpression(expression.Operands[0], environment)
		if operandErr != nil {
			return result, operandErr
		}
		result, err = evaluateUnary(expression.Operator, operand)
	case semanticir.ExprBinary:
		if len(expression.Operands) != 2 {
			return result, fmt.Errorf("%s expression has %d operands", expression.Kind, len(expression.Operands))
		}
		left, leftErr := evaluateExpression(expression.Operands[0], environment)
		if leftErr != nil {
			return result, leftErr
		}
		right, rightErr := evaluateExpression(expression.Operands[1], environment)
		if rightErr != nil {
			return result, rightErr
		}
		result, err = evaluateBinary(expression.Operator, left, right)
	case semanticir.ExprCompare:
		if expression.Operator == semanticir.OpIsNull {
			if len(expression.Operands) != 1 {
				return result, fmt.Errorf("is-null comparison has %d operands", len(expression.Operands))
			}
			operand, operandErr := evaluateExpression(expression.Operands[0], environment)
			if operandErr != nil {
				return result, operandErr
			}
			result, err = evaluateUnary(expression.Operator, operand)
			break
		}
		if len(expression.Operands) != 2 {
			return result, fmt.Errorf("comparison expression has %d operands", len(expression.Operands))
		}
		left, leftErr := evaluateExpression(expression.Operands[0], environment)
		if leftErr != nil {
			return result, leftErr
		}
		right, rightErr := evaluateExpression(expression.Operands[1], environment)
		if rightErr != nil {
			return result, rightErr
		}
		result, err = evaluateBinary(expression.Operator, left, right)
	case semanticir.ExprBool:
		if len(expression.Operands) < 2 || (expression.Operator != semanticir.OpAnd && expression.Operator != semanticir.OpOr) {
			return result, fmt.Errorf("boolean expression has invalid operator/arity")
		}
		value := expression.Operator == semanticir.OpAnd
		for _, operandExpression := range expression.Operands {
			operand, operandErr := evaluateExpression(operandExpression, environment)
			if operandErr != nil {
				return result, operandErr
			}
			if operand.Type != semanticir.TypeBool {
				return result, fmt.Errorf("boolean expression has a non-boolean operand")
			}
			if expression.Operator == semanticir.OpAnd && !operand.Bool {
				value = false
				break
			}
			if expression.Operator == semanticir.OpOr && operand.Bool {
				value = true
				break
			}
		}
		result = semanticir.Literal{Type: semanticir.TypeBool, Bool: value}
	case semanticir.ExprField:
		if len(expression.Operands) != 1 || strings.TrimSpace(expression.Name) == "" {
			return result, fmt.Errorf("field projection needs one operand and a field name")
		}
		record, recordErr := evaluateExpression(expression.Operands[0], environment)
		if recordErr != nil {
			return result, recordErr
		}
		if record.Type != semanticir.TypeRecord || record.Fields == nil {
			return result, fmt.Errorf("field projection operand is not a record")
		}
		var exists bool
		result, exists = record.Fields.Values[expression.Name]
		if !exists {
			return result, fmt.Errorf("record has no field %q", expression.Name)
		}
	case semanticir.ExprIndex:
		if len(expression.Operands) != 2 {
			return result, fmt.Errorf("index expression needs two operands")
		}
		collection, collectionErr := evaluateExpression(expression.Operands[0], environment)
		if collectionErr != nil {
			return result, collectionErr
		}
		index, indexErr := evaluateExpression(expression.Operands[1], environment)
		if indexErr != nil {
			return result, indexErr
		}
		result, err = evaluateIndex(collection, index)
	case semanticir.ExprSequence:
		if expression.Type != semanticir.TypeSequence && expression.Type != semanticir.TypeTuple {
			return result, fmt.Errorf("sequence expression has type %q", expression.Type)
		}
		values := make([]semanticir.Literal, len(expression.Operands))
		for i, operand := range expression.Operands {
			values[i], err = evaluateExpression(operand, environment)
			if err != nil {
				return result, err
			}
		}
		result = semanticir.Literal{Type: expression.Type, Elements: &semanticir.LiteralElements{Values: values}}
	case semanticir.ExprRecord:
		fields := make(map[string]semanticir.Literal, len(expression.Operands))
		for _, operand := range expression.Operands {
			if strings.TrimSpace(operand.Name) == "" || fields[operand.Name].Type != "" {
				return result, fmt.Errorf("record expression has empty or duplicate field name %q", operand.Name)
			}
			fields[operand.Name], err = evaluateExpression(operand, environment)
			if err != nil {
				return result, err
			}
		}
		result = semanticir.Literal{Type: semanticir.TypeRecord, Fields: &semanticir.LiteralFields{Values: fields}}
	case semanticir.ExprCall:
		return result, fmt.Errorf("call expression %q has no closed finite evaluator", expression.Name)
	default:
		return result, fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
	if err != nil {
		return result, err
	}
	if result.Type != expression.Type {
		return result, fmt.Errorf("expression %s produced type %q, want %q", expression.Kind, result.Type, expression.Type)
	}
	return result, nil
}

func evaluateUnary(operator semanticir.Operator, operand semanticir.Literal) (semanticir.Literal, error) {
	switch operator {
	case semanticir.OpNot:
		if operand.Type != semanticir.TypeBool {
			return semanticir.Literal{}, fmt.Errorf("not operand is not boolean")
		}
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: !operand.Bool}, nil
	case semanticir.OpNeg:
		if operand.Type != semanticir.TypeInteger || operand.Integer == math.MinInt64 {
			return semanticir.Literal{}, fmt.Errorf("integer negation is invalid or overflows")
		}
		return semanticir.Literal{Type: semanticir.TypeInteger, Integer: -operand.Integer}, nil
	case semanticir.OpIsNull:
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: operand.Type == semanticir.TypeOptional && operand.Null}, nil
	default:
		return semanticir.Literal{}, fmt.Errorf("unsupported unary operator %q", operator)
	}
}

func evaluateBinary(operator semanticir.Operator, left, right semanticir.Literal) (semanticir.Literal, error) {
	switch operator {
	case semanticir.OpEQ, semanticir.OpNE:
		equal := reflect.DeepEqual(left, right)
		if operator == semanticir.OpNE {
			equal = !equal
		}
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: equal}, nil
	case semanticir.OpAnd, semanticir.OpOr:
		if left.Type != semanticir.TypeBool || right.Type != semanticir.TypeBool {
			return semanticir.Literal{}, fmt.Errorf("boolean operator has non-boolean operand")
		}
		value := left.Bool && right.Bool
		if operator == semanticir.OpOr {
			value = left.Bool || right.Bool
		}
		return semanticir.Literal{Type: semanticir.TypeBool, Bool: value}, nil
	case semanticir.OpLT, semanticir.OpLE, semanticir.OpGT, semanticir.OpGE:
		return compareOrdered(operator, left, right)
	case semanticir.OpIn:
		return evaluateMembership(left, right)
	case semanticir.OpAdd:
		if left.Type == semanticir.TypeString && right.Type == semanticir.TypeString {
			return semanticir.Literal{Type: semanticir.TypeString, String: left.String + right.String}, nil
		}
		if (left.Type == semanticir.TypeSequence || left.Type == semanticir.TypeTuple) && right.Type == left.Type && left.Elements != nil && right.Elements != nil {
			values := append([]semanticir.Literal(nil), left.Elements.Values...)
			values = append(values, right.Elements.Values...)
			return semanticir.Literal{Type: left.Type, Elements: &semanticir.LiteralElements{Values: values}}, nil
		}
		return integerOperation(operator, left, right)
	case semanticir.OpSub, semanticir.OpMul, semanticir.OpDiv, semanticir.OpMod:
		return integerOperation(operator, left, right)
	default:
		return semanticir.Literal{}, fmt.Errorf("unsupported binary operator %q", operator)
	}
}

func integerOperation(operator semanticir.Operator, left, right semanticir.Literal) (semanticir.Literal, error) {
	if left.Type != semanticir.TypeInteger || right.Type != semanticir.TypeInteger {
		return semanticir.Literal{}, fmt.Errorf("integer operator has non-integer operand")
	}
	if (operator == semanticir.OpDiv || operator == semanticir.OpMod) && right.Integer == 0 {
		return semanticir.Literal{}, fmt.Errorf("division by zero")
	}
	if operator == semanticir.OpDiv {
		if left.Integer == math.MinInt64 && right.Integer == -1 {
			return semanticir.Literal{}, fmt.Errorf("integer division overflows")
		}
		return semanticir.Literal{Type: semanticir.TypeInteger, Integer: left.Integer / right.Integer}, nil
	}
	if operator == semanticir.OpMod {
		return semanticir.Literal{Type: semanticir.TypeInteger, Integer: left.Integer % right.Integer}, nil
	}
	a := big.NewInt(left.Integer)
	b := big.NewInt(right.Integer)
	switch operator {
	case semanticir.OpAdd:
		a.Add(a, b)
	case semanticir.OpSub:
		a.Sub(a, b)
	case semanticir.OpMul:
		a.Mul(a, b)
	}
	if !a.IsInt64() {
		return semanticir.Literal{}, fmt.Errorf("integer operation overflows")
	}
	return semanticir.Literal{Type: semanticir.TypeInteger, Integer: a.Int64()}, nil
}

func compareOrdered(operator semanticir.Operator, left, right semanticir.Literal) (semanticir.Literal, error) {
	comparison := 0
	switch {
	case left.Type == semanticir.TypeInteger && right.Type == semanticir.TypeInteger:
		if left.Integer < right.Integer {
			comparison = -1
		} else if left.Integer > right.Integer {
			comparison = 1
		}
	case left.Type == semanticir.TypeString && right.Type == semanticir.TypeString:
		comparison = strings.Compare(left.String, right.String)
	default:
		return semanticir.Literal{}, fmt.Errorf("ordered comparison has incompatible operands")
	}
	value := comparison < 0
	switch operator {
	case semanticir.OpLE:
		value = comparison <= 0
	case semanticir.OpGT:
		value = comparison > 0
	case semanticir.OpGE:
		value = comparison >= 0
	}
	return semanticir.Literal{Type: semanticir.TypeBool, Bool: value}, nil
}

func evaluateMembership(left, right semanticir.Literal) (semanticir.Literal, error) {
	value := false
	switch right.Type {
	case semanticir.TypeSequence, semanticir.TypeTuple:
		if right.Elements == nil {
			return semanticir.Literal{}, fmt.Errorf("membership collection has no elements")
		}
		for _, candidate := range right.Elements.Values {
			if reflect.DeepEqual(left, candidate) {
				value = true
				break
			}
		}
	case semanticir.TypeRecord:
		if left.Type != semanticir.TypeString || right.Fields == nil {
			return semanticir.Literal{}, fmt.Errorf("record membership requires a string key")
		}
		_, value = right.Fields.Values[left.String]
	case semanticir.TypeString:
		if left.Type != semanticir.TypeString {
			return semanticir.Literal{}, fmt.Errorf("string membership requires a string operand")
		}
		value = strings.Contains(right.String, left.String)
	default:
		return semanticir.Literal{}, fmt.Errorf("unsupported membership collection type %q", right.Type)
	}
	return semanticir.Literal{Type: semanticir.TypeBool, Bool: value}, nil
}

func evaluateIndex(collection, index semanticir.Literal) (semanticir.Literal, error) {
	switch collection.Type {
	case semanticir.TypeSequence, semanticir.TypeTuple:
		if collection.Elements == nil || index.Type != semanticir.TypeInteger || index.Integer < 0 || index.Integer >= int64(len(collection.Elements.Values)) {
			return semanticir.Literal{}, fmt.Errorf("sequence index is invalid or out of range")
		}
		return collection.Elements.Values[index.Integer], nil
	case semanticir.TypeRecord:
		if collection.Fields == nil || index.Type != semanticir.TypeString {
			return semanticir.Literal{}, fmt.Errorf("record index is not a string")
		}
		value, exists := collection.Fields.Values[index.String]
		if !exists {
			return semanticir.Literal{}, fmt.Errorf("record has no key %q", index.String)
		}
		return value, nil
	default:
		return semanticir.Literal{}, fmt.Errorf("type %q is not indexable", collection.Type)
	}
}
