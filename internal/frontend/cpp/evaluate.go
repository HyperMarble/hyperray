package cpp

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

const maxFiniteAssignments = 100_000

type runtimeValue struct {
	typeName semanticir.ValueType
	b        bool
	i        int64
	s        string
}

type terminal struct {
	kind          semanticir.OutcomeKind
	value         *runtimeValue
	exceptionType string
	message       string
	effects       []semanticir.Effect
	provenance    semanticir.Provenance
}

func (l *lowerer) enumerateCases() {
	if semanticir.HasErrors(l.diagnostics) {
		return
	}
	operationMap := operationLookup(l.operations)
	outcomeIndex := make(map[string]bool)
	for operationIndex := range l.operations {
		operation := l.operations[operationIndex]
		if operation.operation.Kind == semanticir.OperationTest {
			continue
		}
		assignments, ok := l.assignmentsFor(operation.operation)
		if !ok {
			continue
		}
		for index, assignment := range assignments {
			if l.assignmentExcluded(operation.operation.ID, assignment) {
				continue
			}
			environment, ok := l.environmentFor(operation.operation, assignment)
			if !ok {
				continue
			}
			inputs, exact := l.exactInputsForAssignment(operation.operation, assignment)
			if !exact {
				continue
			}
			result, terminated, err := l.execute(operation.operation.Body, environment, operationMap, 0)
			if err != nil {
				l.blockProvenance(operation.operation.Provenance, "uncontrolled-ub", fmt.Sprintf("operation %s under %s: %v", operation.operation.ID, formatAssignment(assignment), err))
				continue
			}
			if !terminated {
				if operation.resultType != semanticir.TypeUnit {
					l.blockProvenance(operation.operation.Provenance, "missing-terminal", fmt.Sprintf("non-void operation %s can fall through without a return", operation.operation.ID))
					continue
				}
				unit := runtimeValue{typeName: semanticir.TypeUnit}
				result.kind = semanticir.OutcomeReturn
				result.value = &unit
				result.provenance = operation.operation.Provenance
			}
			caseID := fmt.Sprintf("%s:case:%d", operation.operation.ID, index)
			rawOutcome := outcomeFromTerminal(operation.operation.ID, result)
			outcome := rawOutcome
			if len(operation.operation.OutcomeIDs) > 0 {
				mappedID := semanticir.ClassifyOutcome(operation.operation, rawOutcome)
				if !containsString(operation.operation.OutcomeIDs, mappedID) {
					l.blockProvenance(result.provenance, "outcome-universe", fmt.Sprintf("operation %s compiler-derived outcome %s (%s) maps to undeclared outcome %s", operation.operation.ID, rawOutcome.ID, describeOutcome(rawOutcome), mappedID), semanticir.DiagnosticInvalidReference)
					continue
				}
				if mappedID != rawOutcome.ID {
					outcome = semanticir.OtherOutcome(operation.operation.ID, result.provenance)
				}
			}
			outcome = l.bindDeclaredOutcome(outcome)
			if !outcomeIndex[outcome.ID] {
				outcomeIndex[outcome.ID] = true
				l.outcomes = append(l.outcomes, outcome)
			}
			if !containsString(l.operations[operationIndex].operation.OutcomeIDs, outcome.ID) {
				l.operations[operationIndex].operation.OutcomeIDs = append(l.operations[operationIndex].operation.OutcomeIDs, outcome.ID)
			}
			l.caseRawOutcomes[caseID] = rawOutcome
			l.cases = append(l.cases, semanticir.BehaviorCase{
				ID:          caseID,
				Conditions:  cloneAssignment(assignment),
				OperationID: operation.operation.ID,
				Inputs:      cloneLiteralMap(inputs),
				OutcomeIDs:  []string{outcome.ID},
				Provenance:  result.provenance,
			})
		}
	}
}

func describeOutcome(outcome semanticir.ObservableOutcome) string {
	parts := []string{string(outcome.Kind)}
	if outcome.Value != nil {
		parts = append(parts, fmt.Sprintf("value=%+v", *outcome.Value))
	}
	for _, effect := range outcome.Effects {
		value := ""
		if effect.Value != nil {
			value = fmt.Sprintf("=%+v", *effect.Value)
		}
		parts = append(parts, fmt.Sprintf("%s:%s%s", effect.Kind, effect.Target, value))
	}
	return strings.Join(parts, "; ")
}

func (l *lowerer) assignmentExcluded(operationID string, assignment semanticir.Assignment) bool {
	for _, constraint := range l.request.Constraints {
		if constraint.OperationID != operationID || len(constraint.Conditions) == 0 {
			continue
		}
		matches := true
		for domainID, valueID := range constraint.Conditions {
			if assignment[domainID] != valueID {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func (l *lowerer) bindDeclaredOutcome(actual semanticir.ObservableOutcome) semanticir.ObservableOutcome {
	for _, declared := range l.request.Outcomes {
		if declared.ID != actual.ID {
			continue
		}
		bound := declared
		bound.Provenance = actual.Provenance
		bound.Effects = append([]semanticir.Effect(nil), declared.Effects...)
		for index := range bound.Effects {
			if index < len(actual.Effects) {
				bound.Effects[index].Provenance = actual.Effects[index].Provenance
				if bound.Effects[index].Value != nil {
					value := *bound.Effects[index].Value
					value.Provenance = actual.Effects[index].Provenance
					bound.Effects[index].Value = &value
				}
			}
		}
		return bound
	}
	return actual
}

func operationLookup(operations []loweredOperation) map[string]loweredOperation {
	result := make(map[string]loweredOperation, len(operations)*2)
	shortCounts := make(map[string]int)
	for _, operation := range operations {
		result[operation.operation.ID] = operation
		shortCounts[shortName(operation.operation.ID)]++
	}
	for _, operation := range operations {
		short := shortName(operation.operation.ID)
		if short != operation.operation.ID && shortCounts[short] == 1 {
			result[short] = operation
		}
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

func (l *lowerer) assignmentsFor(operation semanticir.Operation) ([]semanticir.Assignment, bool) {
	result := []semanticir.Assignment{{}}
	for _, domainID := range operation.DomainIDs {
		domain := l.findDomain(domainID)
		if domain == nil || len(domain.Values) == 0 {
			return nil, false
		}
		if len(result) > maxFiniteAssignments/len(domain.Values) {
			l.blockProvenance(operation.Provenance, "domain-product", fmt.Sprintf("operation %s has more than %d finite assignments", operation.ID, maxFiniteAssignments), semanticir.DiagnosticNonFinite)
			return nil, false
		}
		next := make([]semanticir.Assignment, 0, len(result)*len(domain.Values))
		for _, assignment := range result {
			for _, value := range domain.Values {
				copy := cloneAssignment(assignment)
				copy[domain.ID] = value.ID
				next = append(next, copy)
			}
		}
		result = next
	}
	return result, true
}

func (l *lowerer) findDomain(id string) *semanticir.Domain {
	for i := range l.request.FiniteDomains {
		if l.request.FiniteDomains[i].ID == id {
			return &l.request.FiniteDomains[i]
		}
	}
	return nil
}

func (l *lowerer) environmentFor(operation semanticir.Operation, assignment semanticir.Assignment) (map[string]runtimeValue, bool) {
	environment := make(map[string]runtimeValue, len(operation.Inputs))
	exactInputs, exact := l.exactInputsForAssignment(operation, assignment)
	if !exact {
		l.blockProvenance(operation.Provenance, "non-singleton-grounding", fmt.Sprintf("operation %s assignment does not uniquely fix every compiler input", operation.ID), semanticir.DiagnosticUnsupported)
		return nil, false
	}
	for _, input := range operation.Inputs {
		literal, literalOK := exactInputs[input.Name]
		if !literalOK || literal.Type != input.Type {
			l.blockProvenance(input.Provenance, "domain-value", fmt.Sprintf("assignment has no exact %s literal for input %s", input.Type, input.Name), semanticir.DiagnosticInvalidInput)
			return nil, false
		}
		value := runtimeFromLiteral(literal)
		if input.Type == semanticir.TypeInteger {
			if bits := l.integerBits[provenanceKey(input.Provenance)]; bits == 0 || !fitsSignedBits(value.i, bits) {
				l.blockProvenance(input.Provenance, "domain-value", fmt.Sprintf("input %s is outside the exact signed C++ parameter width", input.Name), semanticir.DiagnosticInvalidInput)
				return nil, false
			}
		}
		environment[input.Name] = value
	}
	return environment, true
}

func (l *lowerer) exactInputsForAssignment(operation semanticir.Operation, assignment semanticir.Assignment) (map[string]semanticir.Literal, bool) {
	if len(l.request.Groundings) != 0 {
		exact, singleton := semanticir.ExactGroundingInputs(operation, l.request.FiniteDomains, assignment)
		if !singleton {
			return nil, false
		}
		matched := false
		for _, grounding := range l.request.Groundings {
			if grounding.OperationID != operation.ID || !assignmentsEqual(grounding.Conditions, assignment) {
				continue
			}
			if matched || !reflect.DeepEqual(exact, grounding.Inputs) {
				return nil, false
			}
			matched = true
		}
		return cloneLiteralMap(exact), matched
	}
	// Compatibility for the old direct-literal fixture vocabulary.
	result := make(map[string]semanticir.Literal, len(operation.Inputs))
	for _, input := range operation.Inputs {
		valueID, exists := assignment[input.DomainID]
		literal, ok := typedDomainLiteralForInput(l.findDomain(input.DomainID), valueID, operation.ID, input.Name)
		if !exists || !ok || literal.Type != input.Type {
			return nil, false
		}
		result[input.Name] = literal
	}
	return result, true
}

func typedDomainLiteral(domain *semanticir.Domain, id string) (semanticir.Literal, bool) {
	if domain == nil {
		return semanticir.Literal{}, false
	}
	for _, value := range domain.Values {
		if value.ID == id {
			return value.TypedValue(*domain)
		}
	}
	return semanticir.Literal{}, false
}

func typedDomainLiteralForInput(domain *semanticir.Domain, id, operationID, inputName string) (semanticir.Literal, bool) {
	if domain == nil {
		return semanticir.Literal{}, false
	}
	for _, value := range domain.Values {
		if value.ID != id {
			continue
		}
		if len(value.Groundings) == 0 {
			return value.TypedValue(*domain)
		}
		var result semanticir.Literal
		found := false
		for _, grounding := range value.Groundings {
			if grounding.OperationID != operationID || grounding.Kind != semanticir.GroundingExact || grounding.Membership != nil {
				continue
			}
			literal, exists := grounding.Exact[inputName]
			if !exists || literal.Type != domain.Type && domain.Type != semanticir.TypeUnknown || found {
				return semanticir.Literal{}, false
			}
			result, found = literal, true
		}
		return result, found
	}
	return semanticir.Literal{}, false
}

func (l *lowerer) execute(statements []semanticir.Statement, environment map[string]runtimeValue, operations map[string]loweredOperation, depth int) (terminal, bool, error) {
	if depth > 64 {
		return terminal{}, false, fmt.Errorf("call depth exceeds 64")
	}
	effects := make([]semanticir.Effect, 0)
	for _, statement := range statements {
		switch statement.Kind {
		case semanticir.StmtBranch:
			if statement.Condition == nil {
				return terminal{}, false, fmt.Errorf("branch is missing condition")
			}
			condition, err := l.eval(*statement.Condition, environment, operations, depth)
			if err != nil {
				return terminal{}, false, err
			}
			if condition.typeName != semanticir.TypeBool {
				return terminal{}, false, fmt.Errorf("branch condition is %s, not bool", condition.typeName)
			}
			body := statement.Else
			if condition.b {
				body = statement.Then
			}
			result, terminated, err := l.execute(body, environment, operations, depth)
			if err != nil {
				return terminal{}, false, err
			}
			if terminated {
				result.effects = append(effects, result.effects...)
				return result, true, nil
			}
			effects = append(effects, result.effects...)
		case semanticir.StmtReturn:
			result := terminal{kind: semanticir.OutcomeReturn, effects: effects, provenance: statement.Provenance}
			if statement.Value != nil {
				value, err := l.eval(*statement.Value, environment, operations, depth)
				if err != nil {
					return terminal{}, false, err
				}
				result.value = &value
			}
			return result, true, nil
		case semanticir.StmtRaise:
			message := statement.Message
			if statement.Value != nil && statement.Value.Type == semanticir.TypeString {
				value, err := l.eval(*statement.Value, environment, operations, depth)
				if err != nil {
					return terminal{}, false, err
				}
				message = value.s
			}
			return terminal{kind: semanticir.OutcomeRaise, exceptionType: statement.ExceptionType, message: message, effects: effects, provenance: statement.Provenance}, true, nil
		case semanticir.StmtCall:
			if statement.Value == nil {
				return terminal{}, false, fmt.Errorf("call statement has no expression")
			}
			if _, exists := operations[statement.Value.Name]; !exists && statement.Value.Type == semanticir.TypeUnit {
				// A resolved external void call is completely represented by the
				// attached EffectCall; it has no return semantics to evaluate.
			} else if _, err := l.eval(*statement.Value, environment, operations, depth); err != nil {
				return terminal{}, false, err
			}
			resolved, err := l.resolveEffects(statement.Effects, environment, operations, depth)
			if err != nil {
				return terminal{}, false, err
			}
			effects = append(effects, resolved...)
		case semanticir.StmtEffect:
			resolved, err := l.resolveEffects(statement.Effects, environment, operations, depth)
			if err != nil {
				return terminal{}, false, err
			}
			effects = append(effects, resolved...)
		default:
			return terminal{}, false, fmt.Errorf("unsupported statement kind %q", statement.Kind)
		}
	}
	return terminal{effects: effects}, false, nil
}

func (l *lowerer) resolveEffects(effects []semanticir.Effect, environment map[string]runtimeValue, operations map[string]loweredOperation, depth int) ([]semanticir.Effect, error) {
	resolved := make([]semanticir.Effect, 0, len(effects))
	for _, effect := range effects {
		copy := effect
		if effect.Value != nil {
			value, err := l.eval(*effect.Value, environment, operations, depth)
			if err != nil {
				return nil, fmt.Errorf("effect %s:%s value: %w", effect.Kind, effect.Target, err)
			}
			literal := literalFromRuntime(value)
			copy.Value = &semanticir.Expression{Kind: semanticir.ExprLiteral, Type: literal.Type, Literal: &literal, Operands: []semanticir.Expression{}, Provenance: effect.Value.Provenance}
		}
		resolved = append(resolved, copy)
	}
	return resolved, nil
}

func (l *lowerer) eval(expression semanticir.Expression, environment map[string]runtimeValue, operations map[string]loweredOperation, depth int) (runtimeValue, error) {
	switch expression.Kind {
	case semanticir.ExprLiteral:
		if expression.Literal == nil {
			return runtimeValue{}, fmt.Errorf("nil literal")
		}
		return runtimeFromLiteral(*expression.Literal), nil
	case semanticir.ExprVariable:
		value, ok := environment[expression.Name]
		if !ok {
			return runtimeValue{}, fmt.Errorf("unbound variable %q", expression.Name)
		}
		return value, nil
	case semanticir.ExprUnary:
		if len(expression.Operands) != 1 {
			return runtimeValue{}, fmt.Errorf("unary expression arity")
		}
		value, err := l.eval(expression.Operands[0], environment, operations, depth)
		if err != nil {
			return runtimeValue{}, err
		}
		switch expression.Operator {
		case semanticir.OpNot:
			if value.typeName != semanticir.TypeBool {
				return runtimeValue{}, fmt.Errorf("not on non-bool")
			}
			return runtimeValue{typeName: semanticir.TypeBool, b: !value.b}, nil
		case semanticir.OpNeg:
			if value.typeName != semanticir.TypeInteger {
				return runtimeValue{}, fmt.Errorf("neg on non-integer")
			}
			bits := l.integerBits[provenanceKey(expression.Provenance)]
			if bits == 0 || value.i == signedMin(bits) {
				return runtimeValue{}, fmt.Errorf("signed negation overflow (undefined behavior)")
			}
			return runtimeValue{typeName: semanticir.TypeInteger, i: -value.i}, nil
		default:
			return runtimeValue{}, fmt.Errorf("unknown unary operator %q", expression.Operator)
		}
	case semanticir.ExprBinary, semanticir.ExprCompare, semanticir.ExprBool:
		if len(expression.Operands) != 2 {
			return runtimeValue{}, fmt.Errorf("binary expression arity")
		}
		left, err := l.eval(expression.Operands[0], environment, operations, depth)
		if err != nil {
			return runtimeValue{}, err
		}
		if expression.Operator == semanticir.OpAnd && left.typeName == semanticir.TypeBool && !left.b {
			return runtimeValue{typeName: semanticir.TypeBool, b: false}, nil
		}
		if expression.Operator == semanticir.OpOr && left.typeName == semanticir.TypeBool && left.b {
			return runtimeValue{typeName: semanticir.TypeBool, b: true}, nil
		}
		right, err := l.eval(expression.Operands[1], environment, operations, depth)
		if err != nil {
			return runtimeValue{}, err
		}
		return evalBinary(expression.Operator, left, right, l.integerBits[provenanceKey(expression.Provenance)])
	case semanticir.ExprCall:
		operation, ok := operations[expression.Name]
		if !ok {
			return runtimeValue{}, fmt.Errorf("call target %q is outside the translated bounded scope", expression.Name)
		}
		if len(expression.Operands) != len(operation.operation.Inputs) {
			return runtimeValue{}, fmt.Errorf("call %q argument count does not match translated operation", expression.Name)
		}
		calleeEnvironment := make(map[string]runtimeValue, len(expression.Operands))
		for index, operand := range expression.Operands {
			value, err := l.eval(operand, environment, operations, depth)
			if err != nil {
				return runtimeValue{}, err
			}
			calleeEnvironment[operation.operation.Inputs[index].Name] = value
		}
		result, terminated, err := l.execute(operation.operation.Body, calleeEnvironment, operations, depth+1)
		if err != nil {
			return runtimeValue{}, err
		}
		if !terminated || result.kind != semanticir.OutcomeReturn || result.value == nil {
			if operation.resultType == semanticir.TypeUnit {
				return runtimeValue{typeName: semanticir.TypeUnit}, nil
			}
			return runtimeValue{}, fmt.Errorf("call %q did not return a value", expression.Name)
		}
		return *result.value, nil
	default:
		return runtimeValue{}, fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
}

func evalBinary(operator semanticir.Operator, left, right runtimeValue, bits int) (runtimeValue, error) {
	if operator == semanticir.OpAdd && left.typeName == semanticir.TypeString && right.typeName == semanticir.TypeString {
		return runtimeValue{typeName: semanticir.TypeString, s: left.s + right.s}, nil
	}
	if operator == semanticir.OpEQ || operator == semanticir.OpNE {
		equal, err := runtimeEqual(left, right)
		if err != nil {
			return runtimeValue{}, err
		}
		if operator == semanticir.OpNE {
			equal = !equal
		}
		return runtimeValue{typeName: semanticir.TypeBool, b: equal}, nil
	}
	if operator == semanticir.OpAnd || operator == semanticir.OpOr {
		if left.typeName != semanticir.TypeBool || right.typeName != semanticir.TypeBool {
			return runtimeValue{}, fmt.Errorf("boolean operator on non-bool")
		}
		if operator == semanticir.OpAnd {
			return runtimeValue{typeName: semanticir.TypeBool, b: left.b && right.b}, nil
		}
		return runtimeValue{typeName: semanticir.TypeBool, b: left.b || right.b}, nil
	}
	if left.typeName != semanticir.TypeInteger || right.typeName != semanticir.TypeInteger {
		return runtimeValue{}, fmt.Errorf("numeric operator on non-integer")
	}
	switch operator {
	case semanticir.OpAdd:
		return checkedArithmetic("addition", bits, left.i, right.i, func(a, b *big.Int) *big.Int { return new(big.Int).Add(a, b) })
	case semanticir.OpSub:
		return checkedArithmetic("subtraction", bits, left.i, right.i, func(a, b *big.Int) *big.Int { return new(big.Int).Sub(a, b) })
	case semanticir.OpMul:
		return checkedArithmetic("multiplication", bits, left.i, right.i, func(a, b *big.Int) *big.Int { return new(big.Int).Mul(a, b) })
	case semanticir.OpDiv, semanticir.OpMod:
		if right.i == 0 {
			return runtimeValue{}, fmt.Errorf("integer divide by zero (undefined behavior)")
		}
		if bits == 0 || left.i == signedMin(bits) && right.i == -1 {
			return runtimeValue{}, fmt.Errorf("signed division overflow (undefined behavior)")
		}
		if operator == semanticir.OpDiv {
			return runtimeValue{typeName: semanticir.TypeInteger, i: left.i / right.i}, nil
		}
		return runtimeValue{typeName: semanticir.TypeInteger, i: left.i % right.i}, nil
	case semanticir.OpLT:
		return runtimeValue{typeName: semanticir.TypeBool, b: left.i < right.i}, nil
	case semanticir.OpLE:
		return runtimeValue{typeName: semanticir.TypeBool, b: left.i <= right.i}, nil
	case semanticir.OpGT:
		return runtimeValue{typeName: semanticir.TypeBool, b: left.i > right.i}, nil
	case semanticir.OpGE:
		return runtimeValue{typeName: semanticir.TypeBool, b: left.i >= right.i}, nil
	default:
		return runtimeValue{}, fmt.Errorf("unknown binary operator %q", operator)
	}
}

func checkedArithmetic(name string, bits int, left, right int64, operation func(*big.Int, *big.Int) *big.Int) (runtimeValue, error) {
	if bits <= 0 || bits > 64 {
		return runtimeValue{}, fmt.Errorf("unknown exact signed integer width")
	}
	result := operation(big.NewInt(left), big.NewInt(right))
	minimum := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)))
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)), big.NewInt(1))
	if result.Cmp(minimum) < 0 || result.Cmp(maximum) > 0 {
		return runtimeValue{}, fmt.Errorf("signed %s overflow (undefined behavior)", name)
	}
	return runtimeValue{typeName: semanticir.TypeInteger, i: result.Int64()}, nil
}

func signedMin(bits int) int64 {
	if bits >= 64 {
		return math.MinInt64
	}
	return -int64(1) << uint(bits-1)
}

func fitsSignedBits(value int64, bits int) bool {
	if bits <= 0 || bits > 64 {
		return false
	}
	if bits == 64 {
		return true
	}
	minimum := signedMin(bits)
	maximum := int64(1)<<uint(bits-1) - 1
	return value >= minimum && value <= maximum
}

func runtimeEqual(left, right runtimeValue) (bool, error) {
	if left.typeName != right.typeName {
		return false, fmt.Errorf("cannot compare %s and %s", left.typeName, right.typeName)
	}
	switch left.typeName {
	case semanticir.TypeBool:
		return left.b == right.b, nil
	case semanticir.TypeInteger:
		return left.i == right.i, nil
	case semanticir.TypeString:
		return left.s == right.s, nil
	case semanticir.TypeUnit:
		return true, nil
	default:
		return false, fmt.Errorf("cannot compare type %s", left.typeName)
	}
}

func runtimeFromLiteral(literal semanticir.Literal) runtimeValue {
	return runtimeValue{typeName: literal.Type, b: literal.Bool, i: literal.Integer, s: literal.String}
}

func outcomeFromTerminal(operationID string, result terminal) semanticir.ObservableOutcome {
	outcome := semanticir.ObservableOutcome{Kind: result.kind, ExceptionType: result.exceptionType, Message: result.message, OperationID: operationID, Effects: append([]semanticir.Effect(nil), result.effects...), Provenance: result.provenance}
	if result.value != nil {
		literal := literalFromRuntime(*result.value)
		outcome.Value = &literal
	}
	outcome.ID = semanticir.OutcomeID(outcome)
	return outcome
}

func literalFromRuntime(value runtimeValue) semanticir.Literal {
	return semanticir.Literal{Type: value.typeName, Bool: value.b, Integer: value.i, String: value.s}
}

func cloneAssignment(assignment semanticir.Assignment) semanticir.Assignment {
	result := make(semanticir.Assignment, len(assignment))
	for key, value := range assignment {
		result[key] = value
	}
	return result
}

func formatAssignment(assignment semanticir.Assignment) string {
	keys := make([]string, 0, len(assignment))
	for key := range assignment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+assignment[key])
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func shortName(id string) string {
	if index := strings.LastIndex(id, "::"); index >= 0 {
		return id[index+2:]
	}
	return id
}

func (l *lowerer) blockProvenance(provenance semanticir.Provenance, kind, reason string, codes ...semanticir.DiagnosticCode) {
	key := fmt.Sprintf("%s:%d:%d:%s", kind, provenance.Location.StartLine, provenance.Location.StartColumn, reason)
	if l.blockedKeys[key] {
		return
	}
	l.blockedKeys[key] = true
	provenance.Translation = semanticir.TranslationUnsupported
	l.total++
	l.unsupported = append(l.unsupported, semanticir.UnsupportedConstruct{Kind: kind, Reason: reason, Provenance: provenance})
	code := semanticir.DiagnosticUnsupported
	if len(codes) > 0 {
		code = codes[0]
	}
	l.diagnostics = append(l.diagnostics, semanticir.Diagnostic{Severity: semanticir.SeverityError, Code: code, Message: reason, Provenance: provenance})
}
