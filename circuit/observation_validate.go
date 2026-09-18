// Observation validation rejects duplicate names and undeclared reads.
// It never permits an opaque Boolean operation in a relation.
package circuit

func observationsError(
	observations []Observation,
	declarations map[string]uint16,
) error {
	names := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		if err := identifierError(observation.name, "observation.name"); err != nil {
			return err
		}
		if _, exists := names[observation.name]; exists {
			return engineError("duplicate_observation", observation.name)
		}
		if err := booleanReferencesError(observation.value, declarations); err != nil {
			return err
		}
		names[observation.name] = struct{}{}
	}
	return nil
}

func booleanReferencesError(
	expression BooleanExpression,
	declarations map[string]uint16,
) error {
	if err := booleanError(expression); err != nil {
		return err
	}
	if err := expressionReferencesError(expression.left.node, declarations); err != nil {
		return err
	}
	return expressionReferencesError(expression.right.node, declarations)
}

func expressionReferencesError(node *bitVectorNode, declarations map[string]uint16) error {
	if node == nil {
		return engineError("unknown_bit_vector_semantics", "node")
	}
	switch node.operation {
	case readVariable:
		width, exists := declarations[node.variable.name]
		if !exists {
			return engineError("undeclared_variable", node.variable.name)
		}
		if width != node.variable.width {
			return engineError("variable_width_mismatch", node.variable.name)
		}
		return nil
	case subtractValues:
		if err := expressionReferencesError(node.left, declarations); err != nil {
			return err
		}
		return expressionReferencesError(node.right, declarations)
	default:
		return engineError("unknown_bit_vector_semantics", "operation")
	}
}
