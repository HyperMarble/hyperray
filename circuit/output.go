// Outputs name next-state values and visible Boolean observations.
// An output never changes the type or width of its expression.
package circuit

// NextState assigns one expression to one next-state variable.
type NextState struct {
	name  string
	value BitVectorExpression
}

// NewNextState builds one named next-state assignment.
func NewNextState(name string, value BitVectorExpression) (NextState, error) {
	if err := identifierError(name, "next_state.name"); err != nil {
		return NextState{}, err
	}
	if _, err := expressionWidth(value); err != nil {
		return NextState{}, err
	}
	return NextState{name: name, value: value}, nil
}

// Observation names one visible Boolean result of a step.
type Observation struct {
	name  string
	value BooleanExpression
}

// NewObservation builds one named Boolean observation.
func NewObservation(name string, value BooleanExpression) (Observation, error) {
	if err := identifierError(name, "observation.name"); err != nil {
		return Observation{}, err
	}
	if err := booleanError(value); err != nil {
		return Observation{}, err
	}
	return Observation{name: name, value: value}, nil
}
