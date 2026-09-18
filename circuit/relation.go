// A step relation joins state, nondeterministic inputs, and typed outputs.
// It never permits an implicit state update or an undeclared input read.
package circuit

// StepRelation is one closed finite bit-vector transition step.
type StepRelation struct {
	states       []Variable
	inputs       []Variable
	nextStates   []NextState
	observations []Observation
}

// NewStepRelation builds a validated and canonically ordered relation.
func NewStepRelation(
	states []Variable,
	inputs []Variable,
	nextStates []NextState,
	observations []Observation,
) (StepRelation, error) {
	relation := StepRelation{
		states:       append([]Variable(nil), states...),
		inputs:       append([]Variable(nil), inputs...),
		nextStates:   append([]NextState(nil), nextStates...),
		observations: append([]Observation(nil), observations...),
	}
	orderRelation(&relation)
	if err := relationError(relation); err != nil {
		return StepRelation{}, err
	}
	return relation, nil
}
