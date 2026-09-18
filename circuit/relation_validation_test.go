// Relation tests reject incomplete state updates and invalid domains.
// They never permit implicit state or duplicated variable ownership.
package circuit_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestRelationValidationRejectsMissingStructure(t *testing.T) {
	_, err := circuit.NewStepRelation(nil, nil, nil, nil)
	requireEngineError(t, err, "no_outputs", "relation")
	_, err = circuit.NewStepRelation([]circuit.Variable{{}}, nil, nil, nil)
	requireEngineError(t, err, "empty_identifier", "relation.variable")
	state := mustVariable(t, "state", 8)
	_, err = circuit.NewStepRelation([]circuit.Variable{state}, nil, nil, nil)
	requireEngineError(t, err, "incomplete_next_state", "relation")
	_, err = circuit.NewStepRelation([]circuit.Variable{state}, []circuit.Variable{state}, nil, nil)
	requireEngineError(t, err, "duplicate_variable", "state")
}

func TestRelationValidationRejectsInvalidNextState(t *testing.T) {
	first := mustVariable(t, "first", 8)
	second := mustVariable(t, "second", 8)
	unknown := mustNext(t, "third", circuit.Read(second))
	outputs := []circuit.NextState{mustNext(t, "first", circuit.Read(first)), unknown}
	_, err := circuit.NewStepRelation([]circuit.Variable{first, second}, nil, outputs, nil)
	requireEngineError(t, err, "unknown_next_state", "third")
	wide := mustVariable(t, "wide", 16)
	output := mustNext(t, "first", circuit.Read(wide))
	_, err = circuit.NewStepRelation([]circuit.Variable{first}, []circuit.Variable{wide},
		[]circuit.NextState{output}, nil)
	requireEngineError(t, err, "width_mismatch", "first")
}

func TestRelationValidationRejectsUndeclaredNextRead(t *testing.T) {
	state := mustVariable(t, "state", 8)
	other := mustVariable(t, "other", 8)
	output := mustNext(t, "state", circuit.Read(other))
	_, err := circuit.NewStepRelation([]circuit.Variable{state}, nil,
		[]circuit.NextState{output}, nil)
	requireEngineError(t, err, "undeclared_variable", "other")
}
