// Expression validation binds every read to the declared step domain.
// It never accepts an unknown node or a width change.
package circuit

func expressionWidth(expression BitVectorExpression) (uint16, error) {
	if expression.node == nil {
		return 0, engineError("unknown_bit_vector_semantics", "expression")
	}
	return nodeWidth(expression.node)
}

func nodeWidth(node *bitVectorNode) (uint16, error) {
	if node == nil {
		return 0, engineError("unknown_bit_vector_semantics", "node")
	}
	switch node.operation {
	case readVariable:
		if err := variableError(node.variable, "expression.variable"); err != nil {
			return 0, err
		}
		return node.variable.width, nil
	case subtractValues:
		return binaryWidth(node)
	default:
		return 0, engineError("unknown_bit_vector_semantics", "operation")
	}
}

func binaryWidth(node *bitVectorNode) (uint16, error) {
	leftWidth, err := nodeWidth(node.left)
	if err != nil {
		return 0, err
	}
	rightWidth, err := nodeWidth(node.right)
	if err != nil {
		return 0, err
	}
	if leftWidth != rightWidth {
		return 0, engineError("width_mismatch", "expression")
	}
	return leftWidth, nil
}

func booleanError(expression BooleanExpression) error {
	if expression.operation != unsignedLessThan && expression.operation != unsignedLessOrEqual {
		return engineError("unknown_boolean_semantics", "expression")
	}
	_, err := comparison(expression.operation, expression.left, expression.right)
	return err
}
