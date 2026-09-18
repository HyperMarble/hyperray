// Observation tests reject duplicate outputs and invalid variable bindings.
// They never infer a Boolean result for an undeclared value.
package circuit_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestRelationValidationRejectsInvalidObservations(t *testing.T) {
	input := mustVariable(t, "input", 8)
	other := mustVariable(t, "other", 8)
	undeclared := mustObservation(t, "flag", selfComparison(t, other))
	_, err := circuit.NewStepRelation(nil, []circuit.Variable{input}, nil,
		[]circuit.Observation{undeclared})
	requireEngineError(t, err, "undeclared_variable", "other")
	duplicate := mustObservation(t, "flag", selfComparison(t, input))
	_, err = circuit.NewStepRelation(nil, []circuit.Variable{input}, nil,
		[]circuit.Observation{duplicate, duplicate})
	requireEngineError(t, err, "duplicate_observation", "flag")
	rightOnly := comparisonExpression(t, "unsigned_less_than", circuit.Read(input), circuit.Read(other))
	observation := mustObservation(t, "right", rightOnly)
	_, err = circuit.NewStepRelation(nil, []circuit.Variable{input}, nil,
		[]circuit.Observation{observation})
	requireEngineError(t, err, "undeclared_variable", "other")
}

func TestRelationValidationRejectsReferenceWidthChange(t *testing.T) {
	input := mustVariable(t, "input", 8)
	changed := mustVariable(t, "input", 16)
	observation := mustObservation(t, "flag", selfComparison(t, changed))
	_, err := circuit.NewStepRelation(nil, []circuit.Variable{input}, nil,
		[]circuit.Observation{observation})
	requireEngineError(t, err, "variable_width_mismatch", "input")
}
