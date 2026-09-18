// Expression emission supports only the typed operations in this package.
// It never falls back to an approximate or uninterpreted operation.
package circuit

func bitVectorSMT(node *bitVectorNode, symbols map[string]string) (string, error) {
	if node == nil {
		return "", engineError("unknown_bit_vector_semantics", "node")
	}
	switch node.operation {
	case readVariable:
		symbol, exists := symbols[node.variable.name]
		if !exists {
			return "", engineError("undeclared_variable", node.variable.name)
		}
		return symbol, nil
	case subtractValues:
		return binarySMT("bvsub", node, symbols)
	default:
		return "", engineError("unknown_bit_vector_semantics", "operation")
	}
}

func binarySMT(operation string, node *bitVectorNode, symbols map[string]string) (string, error) {
	left, err := bitVectorSMT(node.left, symbols)
	if err != nil {
		return "", err
	}
	right, err := bitVectorSMT(node.right, symbols)
	if err != nil {
		return "", err
	}
	return "(" + operation + " " + left + " " + right + ")", nil
}

func booleanSMT(expression BooleanExpression, symbols map[string]string) (string, error) {
	operation := ""
	switch expression.operation {
	case unsignedLessThan:
		operation = "bvult"
	case unsignedLessOrEqual:
		operation = "bvule"
	default:
		return "", engineError("unknown_boolean_semantics", "operation")
	}
	left, err := bitVectorSMT(expression.left.node, symbols)
	if err != nil {
		return "", err
	}
	right, err := bitVectorSMT(expression.right.node, symbols)
	if err != nil {
		return "", err
	}
	return "(" + operation + " " + left + " " + right + ")", nil
}
