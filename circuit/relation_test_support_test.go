// Test constructors keep invalid-relation cases short and explicit.
// They never hide a production error behind a default output.
package circuit_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func mustNext(
	t *testing.T,
	name string,
	value circuit.BitVectorExpression,
) circuit.NextState {
	t.Helper()
	output, err := circuit.NewNextState(name, value)
	if err != nil {
		t.Fatalf("NewNextState(%q) error = %v", name, err)
	}
	return output
}

func mustObservation(
	t *testing.T,
	name string,
	value circuit.BooleanExpression,
) circuit.Observation {
	t.Helper()
	output, err := circuit.NewObservation(name, value)
	if err != nil {
		t.Fatalf("NewObservation(%q) error = %v", name, err)
	}
	return output
}

func selfComparison(t *testing.T, variable circuit.Variable) circuit.BooleanExpression {
	t.Helper()
	value, err := circuit.UnsignedLessThan(circuit.Read(variable), circuit.Read(variable))
	if err != nil {
		t.Fatalf("UnsignedLessThan() error = %v", err)
	}
	return value
}

func namedRelation(
	t *testing.T,
	state circuit.Variable,
	input circuit.Variable,
	observationName string,
) circuit.StepRelation {
	t.Helper()
	relation, err := circuit.NewStepRelation(
		[]circuit.Variable{state}, []circuit.Variable{input},
		[]circuit.NextState{mustNext(t, state.Name(), circuit.Read(state))},
		[]circuit.Observation{mustObservation(t, observationName, selfComparison(t, state))},
	)
	if err != nil {
		t.Fatalf("NewStepRelation() error = %v", err)
	}
	return relation
}
