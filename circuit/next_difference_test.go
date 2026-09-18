// This test measures a next-state difference without a Boolean observation.
// It never assumes that only visible flags can differ.
package circuit_test

import (
	"context"
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestSingleNextStateDifference(t *testing.T) {
	balance := mustVariable(t, "balance", 8)
	cost := mustVariable(t, "cost", 8)
	reference := nextOnlyRelation(t, balance, cost, circuit.Read(balance))
	remaining, err := circuit.Subtract(circuit.Read(balance), circuit.Read(cost))
	if err != nil {
		t.Fatalf("Subtract() error = %v", err)
	}
	candidate := nextOnlyRelation(t, balance, cost, remaining)
	miter, err := circuit.BuildMiter(reference, candidate)
	if err != nil {
		t.Fatalf("BuildMiter() error = %v", err)
	}
	proposal, err := identifiedEngine(t).Propose(context.Background(), miter)
	if err != nil {
		t.Fatalf("DifferenceEngine.Propose() error = %v", err)
	}
	if proposal.Difference == nil {
		t.Fatal("DifferenceEngine.Propose() returned no difference")
	}
	if proposal.Difference.Kind != circuit.NextStateOutput {
		t.Errorf("Difference.Kind = %q, want %q", proposal.Difference.Kind, circuit.NextStateOutput)
	}
}

func nextOnlyRelation(
	t *testing.T,
	state circuit.Variable,
	input circuit.Variable,
	value circuit.BitVectorExpression,
) circuit.StepRelation {
	t.Helper()
	relation, err := circuit.NewStepRelation(
		[]circuit.Variable{state}, []circuit.Variable{input},
		[]circuit.NextState{mustNext(t, state.Name(), value)}, nil,
	)
	if err != nil {
		t.Fatalf("NewStepRelation() error = %v", err)
	}
	return relation
}
