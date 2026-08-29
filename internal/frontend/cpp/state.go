package cpp

import (
	"fmt"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// lowerLocalDeclaration supports scalar bindings represented as symbolic SSA
// values. Mutations are admitted only by the bounded-loop helpers below,
// which replace the binding with a new expression at each proven iteration.
func (l *lowerer) lowerLocalDeclaration(node *astNode) []semanticir.Statement {
	if len(node.Inner) != 1 || node.Inner[0] == nil || node.Inner[0].Kind != "VarDecl" {
		l.block(node, "local-declaration", "only one initialized local binding can be lowered exactly")
		return nil
	}
	declaration := node.Inner[0]
	if len(declaration.Inner) != 1 {
		l.block(node, "local-declaration", "local binding does not have exactly one initializer")
		return nil
	}
	if !strings.Contains(declaration.Type.QualType, "*") {
		initializer := unwrapExpression(declaration.Inner[0])
		if initializer != nil && (initializer.Kind == "ConditionalOperator" || initializer.Kind == "BinaryConditionalOperator") {
			l.block(node, "conditional-local", "conditional scalar bindings must be expanded with their remaining lexical scope")
			return nil
		}
		declaredType, ok := l.valueType(declaration.Type.QualType)
		if !ok || declaredType == semanticir.TypeUnit {
			l.block(node, "local-declaration", fmt.Sprintf("local %q has unsupported scalar type %q", declaration.Name, declaration.Type.QualType))
			return nil
		}
		value, ok := l.lowerExpression(declaration.Inner[0])
		if !ok || value.Type != declaredType {
			if ok {
				l.block(node, "local-declaration", fmt.Sprintf("local %q initializer has type %s, want %s", declaration.Name, value.Type, declaredType))
			}
			return nil
		}
		l.bindLocalExpression(declaration, value)
		l.accept()
		return nil
	}
	l.block(node, "pointer-local", "pointer/reference local construction, aliasing, lifetime, and pointee identity are not represented by the generic bounded frontend")
	return nil
}

func (l *lowerer) bindLocalExpression(declaration *astNode, expression semanticir.Expression) {
	if declaration == nil {
		return
	}
	if declaration.ID != "" {
		l.localExpressions[declaration.ID] = expression
	}
	if declaration.Name != "" {
		l.localExpressions[declaration.Name] = expression
	}
}

func (l *lowerer) localBinding(reference *astNode) (semanticir.Expression, string, string, bool) {
	if reference == nil || reference.Kind != "DeclRefExpr" || reference.ReferencedDecl.Kind != "VarDecl" {
		return semanticir.Expression{}, "", "", false
	}
	id, name := reference.ReferencedDecl.ID, reference.ReferencedDecl.Name
	if value, ok := l.localExpressions[id]; ok {
		return value, id, name, true
	}
	if value, ok := l.localExpressions[name]; ok {
		return value, id, name, true
	}
	return semanticir.Expression{}, "", "", false
}

func (l *lowerer) replaceLocalBinding(id, name string, value semanticir.Expression) {
	if id != "" {
		l.localExpressions[id] = value
	}
	if name != "" {
		l.localExpressions[name] = value
	}
}

func (l *lowerer) lowerLocalAssignment(node *astNode) bool {
	if node == nil || node.Kind != "BinaryOperator" || node.Opcode != "=" || len(node.Inner) != 2 {
		return false
	}
	current, id, name, ok := l.localBinding(node.Inner[0])
	if !ok {
		return false
	}
	value, lowered := l.lowerExpression(node.Inner[1])
	if !lowered || value.Type != current.Type {
		if lowered {
			l.block(node, "local-assignment-type", fmt.Sprintf("assignment to local %q changes %s to %s", name, current.Type, value.Type))
		}
		return true
	}
	l.replaceLocalBinding(id, name, value)
	l.accept()
	return true
}

func (l *lowerer) lowerLocalCompoundAssignment(node *astNode) bool {
	if node == nil || node.Kind != "CompoundAssignOperator" || len(node.Inner) != 2 {
		return false
	}
	current, id, name, ok := l.localBinding(node.Inner[0])
	if !ok {
		return false
	}
	right, lowered := l.lowerExpression(node.Inner[1])
	if !lowered || current.Type != semanticir.TypeInteger || right.Type != semanticir.TypeInteger {
		if lowered {
			l.block(node, "local-compound-type", fmt.Sprintf("compound assignment to local %q is not exact integer arithmetic", name))
		}
		return true
	}
	var operator semanticir.Operator
	switch node.Opcode {
	case "+=":
		operator = semanticir.OpAdd
	case "-=":
		operator = semanticir.OpSub
	case "*=":
		operator = semanticir.OpMul
	case "/=":
		operator = semanticir.OpDiv
	case "%=":
		operator = semanticir.OpMod
	default:
		l.block(node, "local-compound-operator", fmt.Sprintf("compound operator %q is unsupported or has uncontrolled bit semantics", node.Opcode))
		return true
	}
	provenance := l.provenance(node, semanticir.TranslationTranslated)
	value := semanticir.Expression{Kind: semanticir.ExprBinary, Type: semanticir.TypeInteger, Operator: operator, Operands: []semanticir.Expression{current, right}, Provenance: provenance}
	l.recordIntegerWidth(provenance, node.Type.QualType)
	l.replaceLocalBinding(id, name, value)
	l.accept()
	return true
}

func (l *lowerer) lowerLocalIncrement(node *astNode) bool {
	if node == nil || node.Kind != "UnaryOperator" || len(node.Inner) != 1 || node.Opcode != "++" && node.Opcode != "--" {
		return false
	}
	current, id, name, ok := l.localBinding(node.Inner[0])
	if !ok {
		return false
	}
	if current.Type != semanticir.TypeInteger {
		l.block(node, "local-increment-type", fmt.Sprintf("increment/decrement local %q is not an integer", name))
		return true
	}
	operator := semanticir.OpAdd
	if node.Opcode == "--" {
		operator = semanticir.OpSub
	}
	provenance := l.provenance(node, semanticir.TranslationTranslated)
	one := semanticir.Expression{Kind: semanticir.ExprLiteral, Type: semanticir.TypeInteger, Literal: &semanticir.Literal{Type: semanticir.TypeInteger, Integer: 1}, Operands: []semanticir.Expression{}, Provenance: provenance}
	value := semanticir.Expression{Kind: semanticir.ExprBinary, Type: semanticir.TypeInteger, Operator: operator, Operands: []semanticir.Expression{current, one}, Provenance: provenance}
	l.recordIntegerWidth(provenance, node.Type.QualType)
	l.replaceLocalBinding(id, name, value)
	l.accept()
	return true
}

func (l *lowerer) localPointerTruth(node *astNode) (semanticir.Expression, bool) {
	node = unwrapExpression(node)
	if node == nil || node.Kind != "DeclRefExpr" || node.ReferencedDecl.ID == "" {
		return semanticir.Expression{}, false
	}
	expression, exists := l.localExpressions[node.ReferencedDecl.ID]
	return expression, exists
}

func isImplicitThisMember(node *astNode) bool {
	if node == nil || node.Kind != "MemberExpr" || len(node.Inner) != 1 {
		return false
	}
	receiver := unwrapExpression(node.Inner[0])
	return receiver != nil && receiver.Kind == "CXXThisExpr" && (receiver.IsImplicit || receiver.Implicit)
}

func (l *lowerer) lowerStateMember(node *astNode) (semanticir.Expression, bool) {
	if l.currentOperation == nil || node.Name == "" {
		l.block(node, "state-member", "member read is outside a selected bounded operation")
		return semanticir.Expression{}, false
	}
	typeName, ok := l.valueType(node.Type.QualType)
	if !ok {
		// Scoped enums are finite categorical values in the string IR domain.
		if strings.Contains(node.Type.QualType, "enum ") {
			typeName, ok = semanticir.TypeString, true
		}
	}
	if !ok || typeName == semanticir.TypeUnit {
		l.block(node, "state-member-type", fmt.Sprintf("member %q has unsupported state type %q", node.Name, node.Type.QualType))
		return semanticir.Expression{}, false
	}
	domainID := l.stateDomain(node.Name)
	if domainID == "" {
		l.block(node, "missing-finite-domain", fmt.Sprintf("state member %s.%s has no explicit finite domain", l.currentOperation.ID, node.Name), semanticir.DiagnosticMissingDomain)
		return semanticir.Expression{}, false
	}
	domain := l.findDomain(domainID)
	if domain == nil || domain.Type != semanticir.TypeUnknown && domain.Type != typeName {
		l.block(node, "state-domain-type", fmt.Sprintf("state member %s domain %s does not match %s", node.Name, domainID, typeName), semanticir.DiagnosticInvalidInput)
		return semanticir.Expression{}, false
	}
	l.ensureOperationInput(node.Name, typeName, domainID, node)
	l.accept()
	return semanticir.Expression{Kind: semanticir.ExprVariable, Type: typeName, Name: node.Name, Operands: []semanticir.Expression{}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}, true
}

func (l *lowerer) stateDomain(member string) string {
	if l.currentOperation == nil {
		return ""
	}
	for _, declared := range l.request.Operations {
		if declared.ID != l.currentOperation.ID {
			continue
		}
		for _, input := range declared.Inputs {
			if input.Name == member && l.hasDomain(input.DomainID) {
				return input.DomainID
			}
		}
	}
	return ""
}

func (l *lowerer) ensureOperationInput(name string, typeName semanticir.ValueType, domainID string, node *astNode) {
	if l.currentOperation == nil {
		return
	}
	for _, input := range l.currentOperation.Inputs {
		if input.Name == name {
			return
		}
	}
	input := semanticir.Variable{Name: name, Type: typeName, DomainID: domainID, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
	l.currentOperation.Inputs = append(l.currentOperation.Inputs, input)
	if !containsString(l.currentOperation.DomainIDs, domainID) {
		l.currentOperation.DomainIDs = append(l.currentOperation.DomainIDs, domainID)
	}
	l.recordIntegerWidth(input.Provenance, node.Type.QualType)
}

func (l *lowerer) lowerAssignment(node *astNode) []semanticir.Statement {
	if len(node.Inner) != 2 || node.Inner[0] == nil || node.Inner[0].Kind != "MemberExpr" || !isImplicitThisMember(node.Inner[0]) {
		l.block(node, "state-assignment", "assignment target is not an exact member of the current object")
		return nil
	}
	target := node.Inner[0]
	value, ok := l.lowerExpression(node.Inner[1])
	if !ok {
		return nil
	}
	effect := semanticir.Effect{ID: l.effectID(node, target.Name), Kind: semanticir.EffectWrite, Target: target.Name, Value: &value, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
	l.accept()
	return []semanticir.Statement{{Kind: semanticir.StmtEffect, Then: []semanticir.Statement{}, Else: []semanticir.Statement{}, Effects: []semanticir.Effect{effect}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}}
}

func (l *lowerer) lowerOutputStatement(node *astNode) ([]semanticir.Statement, bool) {
	values, ok := outputValues(node)
	if !ok || len(values) == 0 {
		return nil, false
	}
	var combined semanticir.Expression
	for index, valueNode := range values {
		value, lowered := l.lowerExpression(valueNode)
		if !lowered || value.Type != semanticir.TypeString {
			if lowered {
				l.block(valueNode, "output-rendering", fmt.Sprintf("stdout operand has type %s; exact C++ stream formatting is not represented", value.Type))
			}
			return nil, false
		}
		if index == 0 {
			combined = value
		} else {
			combined = semanticir.Expression{Kind: semanticir.ExprBinary, Type: semanticir.TypeString, Operator: semanticir.OpAdd, Operands: []semanticir.Expression{combined, value}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
		}
	}
	effect := semanticir.Effect{ID: fmt.Sprintf("%s:stdout:%d", l.request.Artifact.ID, node.sourceRange().Begin.Offset), Kind: semanticir.EffectOutput, Target: "stdout", Value: &combined, Provenance: l.provenance(node, semanticir.TranslationTranslated)}
	l.accept()
	return []semanticir.Statement{{Kind: semanticir.StmtEffect, Then: []semanticir.Statement{}, Else: []semanticir.Statement{}, Effects: []semanticir.Effect{effect}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}}, true
}

func outputValues(node *astNode) ([]*astNode, bool) {
	if node == nil || node.Kind != "CXXOperatorCallExpr" || len(node.Inner) != 3 || referencedName(node.Inner[0]) != "operator<<" {
		return nil, false
	}
	left := unwrapExpression(node.Inner[1])
	var values []*astNode
	if left != nil && left.Kind == "CXXOperatorCallExpr" {
		var ok bool
		values, ok = outputValues(left)
		if !ok {
			return nil, false
		}
	} else if left == nil || left.Kind != "DeclRefExpr" || left.ReferencedDecl.Name != "cout" {
		return nil, false
	}
	return append(values, node.Inner[2]), true
}

func (l *lowerer) lowerOperatorCall(node *astNode) (semanticir.Expression, bool) {
	if len(node.Inner) != 3 || referencedName(node.Inner[0]) != "operator+" {
		l.block(node, "overloaded-operator", fmt.Sprintf("operator call %q has no closed expression lowering", referencedName(node.Inner[0])))
		return semanticir.Expression{}, false
	}
	left, leftOK := l.lowerExpression(node.Inner[1])
	right, rightOK := l.lowerExpression(node.Inner[2])
	if !leftOK || !rightOK || left.Type != semanticir.TypeString || right.Type != semanticir.TypeString {
		l.block(node, "overloaded-operator", "only resolved std::string concatenation is supported")
		return semanticir.Expression{}, false
	}
	l.accept()
	return semanticir.Expression{Kind: semanticir.ExprBinary, Type: semanticir.TypeString, Operator: semanticir.OpAdd, Operands: []semanticir.Expression{left, right}, Provenance: l.provenance(node, semanticir.TranslationTranslated)}, true
}

func memberCallTarget(node *astNode) string {
	if node == nil || len(node.Inner) == 0 {
		return ""
	}
	callee := node.Inner[0]
	if callee.Kind != "MemberExpr" {
		return referencedName(callee)
	}
	name := callee.Name
	if len(callee.Inner) == 0 {
		return name
	}
	receiver := unwrapExpression(callee.Inner[0])
	if receiver == nil {
		return name
	}
	typeName := receiver.Type.QualType
	typeName = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(strings.TrimPrefix(typeName, "const ")), "*& "))
	if typeName == "" {
		return name
	}
	return typeName + "::" + name
}
