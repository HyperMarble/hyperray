// Relation validation proves structural closure before SMT generation.
// It never treats a missing state assignment as an unchanged state.
package circuit

func relationError(relation StepRelation) error {
	declarations, err := declaredVariables(relation.states, relation.inputs)
	if err != nil {
		return err
	}
	if len(relation.states) == 0 && len(relation.observations) == 0 {
		return engineError("no_outputs", "relation")
	}
	if err := nextStatesError(relation, declarations); err != nil {
		return err
	}
	return observationsError(relation.observations, declarations)
}

func declaredVariables(states []Variable, inputs []Variable) (map[string]uint16, error) {
	declarations := make(map[string]uint16, len(states)+len(inputs))
	for _, variable := range append(append([]Variable(nil), states...), inputs...) {
		if err := variableError(variable, "relation.variable"); err != nil {
			return nil, err
		}
		if _, exists := declarations[variable.name]; exists {
			return nil, engineError("duplicate_variable", variable.name)
		}
		declarations[variable.name] = variable.width
	}
	return declarations, nil
}

func nextStatesError(relation StepRelation, declarations map[string]uint16) error {
	if len(relation.nextStates) != len(relation.states) {
		return engineError("incomplete_next_state", "relation")
	}
	for position, output := range relation.nextStates {
		state := relation.states[position]
		if output.name != state.name {
			return engineError("unknown_next_state", output.name)
		}
		width, err := expressionWidth(output.value)
		if err != nil {
			return err
		}
		if width != state.width {
			return engineError("width_mismatch", output.name)
		}
		if err := expressionReferencesError(output.value.node, declarations); err != nil {
			return err
		}
	}
	return nil
}
