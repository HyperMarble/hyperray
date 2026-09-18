// Ordering tests prove that every public input slice has canonical order.
// They never depend on caller order for miter bytes or output selection.
package circuit_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestPublicMiterIsDeterministicAcrossInputOrder(t *testing.T) {
	first := mustVariable(t, "first", 8)
	second := mustVariable(t, "second", 8)
	one := mustVariable(t, "one", 8)
	two := mustVariable(t, "two", 8)
	firstObservation := mustObservation(t, "first", selfComparison(t, one))
	secondObservation := mustObservation(t, "second", selfComparison(t, two))
	forward := orderedRelation(t,
		[]circuit.Variable{first, second}, []circuit.Variable{one, two},
		[]circuit.NextState{mustNext(t, "first", circuit.Read(first)), mustNext(t, "second", circuit.Read(second))},
		[]circuit.Observation{firstObservation, secondObservation})
	reverse := orderedRelation(t,
		[]circuit.Variable{second, first}, []circuit.Variable{two, one},
		[]circuit.NextState{mustNext(t, "second", circuit.Read(second)), mustNext(t, "first", circuit.Read(first))},
		[]circuit.Observation{secondObservation, firstObservation})
	forwardMiter, forwardError := circuit.BuildMiter(forward, forward)
	reverseMiter, reverseError := circuit.BuildMiter(reverse, reverse)
	if forwardError != nil || reverseError != nil {
		t.Fatalf("BuildMiter() errors = %v, %v", forwardError, reverseError)
	}
	if forwardMiter.SMT2() != reverseMiter.SMT2() {
		t.Error("permuted relations produced different miter bytes")
	}
}

func orderedRelation(
	t *testing.T,
	states []circuit.Variable,
	inputs []circuit.Variable,
	next []circuit.NextState,
	observations []circuit.Observation,
) circuit.StepRelation {
	t.Helper()
	relation, err := circuit.NewStepRelation(states, inputs, next, observations)
	if err != nil {
		t.Fatalf("NewStepRelation() error = %v", err)
	}
	return relation
}
