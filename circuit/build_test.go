// Relation builders exercise only the public circuit construction API.
// They never access package-private expression or relation state.
package circuit_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func balanceRelation(t *testing.T, width uint16, operation string) circuit.StepRelation {
	t.Helper()
	balance := mustVariable(t, "balance", width)
	cost := mustVariable(t, "cost", width)
	remaining, err := circuit.Subtract(circuit.Read(balance), circuit.Read(cost))
	if err != nil {
		t.Fatalf("Subtract() error = %v", err)
	}
	predicate := comparisonExpression(t, operation, circuit.Read(balance), circuit.Read(cost))
	next, err := circuit.NewNextState("balance", remaining)
	if err != nil {
		t.Fatalf("NewNextState() error = %v", err)
	}
	observation, err := circuit.NewObservation("insufficient", predicate)
	if err != nil {
		t.Fatalf("NewObservation() error = %v", err)
	}
	relation, err := circuit.NewStepRelation(
		[]circuit.Variable{balance}, []circuit.Variable{cost},
		[]circuit.NextState{next}, []circuit.Observation{observation},
	)
	if err != nil {
		t.Fatalf("NewStepRelation() error = %v", err)
	}
	return relation
}

func mustVariable(t *testing.T, name string, width uint16) circuit.Variable {
	t.Helper()
	variable, err := circuit.NewVariable(name, width)
	if err != nil {
		t.Fatalf("NewVariable(%q) error = %v", name, err)
	}
	return variable
}

func comparisonExpression(
	t *testing.T,
	operation string,
	left circuit.BitVectorExpression,
	right circuit.BitVectorExpression,
) circuit.BooleanExpression {
	t.Helper()
	switch operation {
	case "unsigned_less_than":
		value, err := circuit.UnsignedLessThan(left, right)
		if err != nil {
			t.Fatalf("UnsignedLessThan() error = %v", err)
		}
		return value
	case "unsigned_less_or_equal":
		value, err := circuit.UnsignedLessOrEqual(left, right)
		if err != nil {
			t.Fatalf("UnsignedLessOrEqual() error = %v", err)
		}
		return value
	default:
		t.Fatalf("unknown comparison %q", operation)
		return circuit.BooleanExpression{}
	}
}
