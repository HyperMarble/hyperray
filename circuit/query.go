// Query records connect stable public names to private SMT symbols.
// They never expose solver-selected internal names as program identities.
package circuit

type assignmentQuery struct {
	role   AssignmentRole
	name   string
	symbol string
	width  uint16
}

type outputQuery struct {
	kind            OutputKind
	name            string
	referenceSymbol string
	candidateSymbol string
	width           uint16
}

func relationAssignments(relation StepRelation) []assignmentQuery {
	assignments := make([]assignmentQuery, 0, len(relation.states)+len(relation.inputs))
	for _, variable := range relation.states {
		assignments = append(assignments, assignmentQuery{
			role: StateAssignment, name: variable.name,
			symbol: stateSymbol(variable.name), width: variable.width,
		})
	}
	for _, variable := range relation.inputs {
		assignments = append(assignments, assignmentQuery{
			role: InputAssignment, name: variable.name,
			symbol: inputSymbol(variable.name), width: variable.width,
		})
	}
	return assignments
}

func relationOutputs(relation StepRelation) []outputQuery {
	outputs := make([]outputQuery, 0, len(relation.nextStates)+len(relation.observations))
	for position, output := range relation.nextStates {
		outputs = append(outputs, outputQuery{
			kind: NextStateOutput, name: output.name,
			referenceSymbol: referenceNextSymbol(position),
			candidateSymbol: candidateNextSymbol(position),
			width:           relation.states[position].width,
		})
	}
	for position, output := range relation.observations {
		outputs = append(outputs, outputQuery{
			kind: ObservationOutput, name: output.name,
			referenceSymbol: referenceObservationSymbol(position),
			candidateSymbol: candidateObservationSymbol(position),
		})
	}
	return outputs
}
