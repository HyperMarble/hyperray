package semanticir

import (
	"fmt"
	"strings"
)

// ValidValueType reports whether valueType belongs to the closed proof value
// vocabulary. TypeUnknown is a translation placeholder and is never valid in
// complete semantic evidence.
func ValidValueType(valueType ValueType) bool {
	switch valueType {
	case TypeBool, TypeInteger, TypeString, TypeUnit, TypeSequence, TypeTuple, TypeRecord, TypeOptional:
		return true
	default:
		return false
	}
}

// ValidateLiteral checks the finite recursive storage invariant for a typed
// literal. Composite cycles are rejected rather than recursed indefinitely.
func ValidateLiteral(literal Literal) error {
	return validateLiteral(&literal, map[any]bool{})
}

func validateLiteral(literal *Literal, seen map[any]bool) error {
	if literal == nil || !ValidValueType(literal.Type) {
		return fmt.Errorf("literal has invalid type %q", literal.Type)
	}
	switch literal.Type {
	case TypeBool, TypeInteger, TypeString, TypeUnit:
		if literal.Null || literal.Elements != nil || literal.Fields != nil {
			return fmt.Errorf("%s literal carries composite or null storage", literal.Type)
		}
	case TypeSequence, TypeTuple:
		if literal.Null || literal.Elements == nil || literal.Fields != nil {
			return fmt.Errorf("%s literal must have element storage only", literal.Type)
		}
		if seen[literal.Elements] {
			return fmt.Errorf("%s literal contains a recursive cycle", literal.Type)
		}
		seen[literal.Elements] = true
		defer delete(seen, literal.Elements)
		for index := range literal.Elements.Values {
			if err := validateLiteral(&literal.Elements.Values[index], seen); err != nil {
				return fmt.Errorf("element %d: %w", index, err)
			}
		}
	case TypeRecord:
		if literal.Null || literal.Fields == nil || literal.Elements != nil {
			return fmt.Errorf("record literal must have field storage only")
		}
		if seen[literal.Fields] {
			return fmt.Errorf("record literal contains a recursive cycle")
		}
		seen[literal.Fields] = true
		defer delete(seen, literal.Fields)
		for name, child := range literal.Fields.Values {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("record literal contains an empty field name")
			}
			childCopy := child
			if err := validateLiteral(&childCopy, seen); err != nil {
				return fmt.Errorf("field %q: %w", name, err)
			}
		}
	case TypeOptional:
		if literal.Fields != nil || (literal.Null && literal.Elements != nil) || (!literal.Null && (literal.Elements == nil || len(literal.Elements.Values) != 1)) {
			return fmt.Errorf("optional literal must be null or contain exactly one element")
		}
		if !literal.Null {
			if seen[literal.Elements] {
				return fmt.Errorf("optional literal contains a recursive cycle")
			}
			seen[literal.Elements] = true
			defer delete(seen, literal.Elements)
			if err := validateLiteral(&literal.Elements.Values[0], seen); err != nil {
				return fmt.Errorf("optional value: %w", err)
			}
		}
	}
	return nil
}

func validateExpression(expression Expression, artifact ArtifactRef, label string) []Diagnostic {
	var diagnostics []Diagnostic
	if err := validateFactSource(expression.Provenance, artifact); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, label+": "+err.Error(), expression.Provenance))
	}
	if !ValidValueType(expression.Type) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("%s has invalid result type %q", label, expression.Type), expression.Provenance))
	}
	plain := expression.Name == "" && expression.Operator == "" && expression.Literal == nil && len(expression.Operands) == 0
	switch expression.Kind {
	case ExprLiteral:
		if expression.Literal == nil || expression.Name != "" || expression.Operator != "" || len(expression.Operands) != 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" literal expression has invalid fields", expression.Provenance))
		} else if err := ValidateLiteral(*expression.Literal); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+": "+err.Error(), expression.Provenance))
		} else if expression.Type != expression.Literal.Type {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("%s type %q differs from literal type %q", label, expression.Type, expression.Literal.Type), expression.Provenance))
		}
	case ExprVariable:
		if strings.TrimSpace(expression.Name) == "" || expression.Operator != "" || expression.Literal != nil || len(expression.Operands) != 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" variable expression has invalid fields", expression.Provenance))
		}
	case ExprUnary:
		if (expression.Operator != OpNot && expression.Operator != OpNeg) || expression.Name != "" || expression.Literal != nil || len(expression.Operands) != 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" unary expression has invalid operator/arity", expression.Provenance))
		}
	case ExprBinary:
		switch expression.Operator {
		case OpAdd, OpSub, OpMul, OpDiv, OpMod:
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("%s has unsupported binary operator %q", label, expression.Operator), expression.Provenance))
		}
		if expression.Name != "" || expression.Literal != nil || len(expression.Operands) != 2 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" binary expression has invalid fields/arity", expression.Provenance))
		}
	case ExprCompare:
		arity := 2
		if expression.Operator == OpIsNull {
			arity = 1
		}
		switch expression.Operator {
		case OpEQ, OpNE, OpLT, OpLE, OpGT, OpGE, OpIn, OpIsNull:
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("%s has unsupported comparison operator %q", label, expression.Operator), expression.Provenance))
		}
		if expression.Type != TypeBool || expression.Name != "" || expression.Literal != nil || len(expression.Operands) != arity {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" comparison expression has invalid fields/type/arity", expression.Provenance))
		}
	case ExprBool:
		if (expression.Operator != OpAnd && expression.Operator != OpOr) || expression.Type != TypeBool || expression.Name != "" || expression.Literal != nil || len(expression.Operands) < 2 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" boolean expression has invalid fields/type/arity", expression.Provenance))
		}
	case ExprCall:
		if strings.TrimSpace(expression.Name) == "" || expression.Operator != "" || expression.Literal != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" call expression has invalid fields", expression.Provenance))
		}
	case ExprField:
		if strings.TrimSpace(expression.Name) == "" || expression.Operator != "" || expression.Literal != nil || len(expression.Operands) != 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" field expression requires a name and one record operand", expression.Provenance))
		}
	case ExprIndex:
		if expression.Name != "" || expression.Operator != "" || expression.Literal != nil || len(expression.Operands) != 2 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" index expression requires sequence and index operands", expression.Provenance))
		}
	case ExprSequence:
		if (expression.Type != TypeSequence && expression.Type != TypeTuple) || expression.Name != "" || expression.Operator != "" || expression.Literal != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" sequence expression has invalid fields/type", expression.Provenance))
		}
	case ExprRecord:
		// Record operands are exact alternating string-literal keys and values.
		if expression.Type != TypeRecord || expression.Name != "" || expression.Operator != "" || expression.Literal != nil || len(expression.Operands)%2 != 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" record expression requires alternating key/value operands", expression.Provenance))
		}
		seen := map[string]struct{}{}
		for index := 0; index+1 < len(expression.Operands); index += 2 {
			key := expression.Operands[index]
			if key.Kind != ExprLiteral || key.Literal == nil || key.Literal.Type != TypeString || key.Literal.String == "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, label+" record keys must be non-empty string literals", key.Provenance))
				continue
			}
			if _, exists := seen[key.Literal.String]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("%s repeats record key %q", label, key.Literal.String), key.Provenance))
			}
			seen[key.Literal.String] = struct{}{}
		}
	default:
		if !plain {
			_ = plain
		}
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("%s has unsupported expression kind %q", label, expression.Kind), expression.Provenance))
	}
	for index := range expression.Operands {
		diagnostics = append(diagnostics, validateExpression(expression.Operands[index], artifact, fmt.Sprintf("%s operand %d", label, index))...)
	}
	return diagnostics
}

func expressionVariables(expression Expression) map[string]struct{} {
	variables := map[string]struct{}{}
	if expression.Kind == ExprVariable && expression.Name != "" {
		variables[expression.Name] = struct{}{}
	}
	for _, operand := range expression.Operands {
		for variable := range expressionVariables(operand) {
			variables[variable] = struct{}{}
		}
	}
	return variables
}

func validateStatements(statements []Statement, artifact ArtifactRef, label string) []Diagnostic {
	var diagnostics []Diagnostic
	for index := range statements {
		statement := statements[index]
		itemLabel := fmt.Sprintf("%s statement %d", label, index)
		if err := validateFactSource(statement.Provenance, artifact); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, itemLabel+": "+err.Error(), statement.Provenance))
		}
		if statement.Condition != nil {
			diagnostics = append(diagnostics, validateExpression(*statement.Condition, artifact, itemLabel+" condition")...)
		}
		if statement.Iterator != nil {
			diagnostics = append(diagnostics, validateExpression(*statement.Iterator, artifact, itemLabel+" iterator")...)
		}
		if statement.Value != nil {
			diagnostics = append(diagnostics, validateExpression(*statement.Value, artifact, itemLabel+" value")...)
		}
		diagnostics = append(diagnostics, validateEffects(statement.Effects, artifact, itemLabel)...)
		switch statement.Kind {
		case StmtBranch:
			if statement.Condition == nil || statement.Condition.Type != TypeBool || len(statement.Then)+len(statement.Else) == 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" branch requires a boolean condition and a body", statement.Provenance))
			}
		case StmtReturn:
			if statement.Condition != nil || statement.Iterator != nil || statement.Target != "" || statement.ExceptionType != "" || len(statement.Catches) != 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" return has unrelated fields", statement.Provenance))
			}
		case StmtRaise:
			if statement.ExceptionType == "" || statement.Condition != nil || statement.Iterator != nil || statement.Target != "" || len(statement.Catches) != 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" raise requires an exception type only", statement.Provenance))
			}
		case StmtCall:
			if statement.Value == nil || statement.Value.Kind != ExprCall {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" call requires a call expression", statement.Provenance))
			}
		case StmtEffect:
			if len(statement.Effects) == 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" effect statement has no effects", statement.Provenance))
			}
		case StmtAssign:
			if statement.Target == "" || statement.Value == nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" assignment requires target and value", statement.Provenance))
			}
		case StmtLoop:
			if statement.Target == "" || statement.Iterator == nil || (statement.Iterator.Type != TypeSequence && statement.Iterator.Type != TypeTuple) || len(statement.Then) == 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" bounded loop requires target, finite sequence/tuple iterator, and body", statement.Provenance))
			}
		case StmtTry:
			if len(statement.Then) == 0 || len(statement.Catches) == 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" try requires body and at least one catch", statement.Provenance))
			}
		case StmtContinue:
			if statement.Condition != nil || statement.Iterator != nil || statement.Value != nil || statement.Target != "" || len(statement.Then)+len(statement.Else)+len(statement.Catches)+len(statement.Effects) != 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" continue has operands", statement.Provenance))
			}
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, fmt.Sprintf("%s has unsupported statement kind %q", itemLabel, statement.Kind), statement.Provenance))
		}
		diagnostics = append(diagnostics, validateStatements(statement.Then, artifact, itemLabel+" then")...)
		diagnostics = append(diagnostics, validateStatements(statement.Else, artifact, itemLabel+" else")...)
		for catchIndex := range statement.Catches {
			clause := statement.Catches[catchIndex]
			if clause.ExceptionType == "" || len(clause.Body) == 0 {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, itemLabel+" catch requires exception type and body", clause.Provenance))
			}
			if err := validateFactSource(clause.Provenance, artifact); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, itemLabel+" catch: "+err.Error(), clause.Provenance))
			}
			diagnostics = append(diagnostics, validateStatements(clause.Body, artifact, itemLabel+" catch")...)
		}
	}
	return diagnostics
}
