// This test measures repeatable engine proposals for one fixed miter.
// It never accepts solver-dependent witness order.
package circuit_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestProposalIsDeterministic(t *testing.T) {
	fixture := fixtureWithStatus(t, string(circuit.DifferenceFound))
	reference := balanceRelation(t, fixture.Width, fixture.ReferenceComparison)
	candidate := balanceRelation(t, fixture.Width, fixture.CandidateComparison)
	miter, err := circuit.BuildMiter(reference, candidate)
	if err != nil {
		t.Fatalf("BuildMiter() error = %v", err)
	}
	tool := identifiedEngine(t)
	first, err := tool.Propose(context.Background(), miter)
	if err != nil {
		t.Fatalf("first DifferenceEngine.Propose() error = %v", err)
	}
	for repetition := 0; repetition < 3; repetition++ {
		proposal, proposalError := tool.Propose(context.Background(), miter)
		if proposalError != nil {
			t.Fatalf("DifferenceEngine.Propose() error = %v", proposalError)
		}
		if !reflect.DeepEqual(proposal, first) {
			t.Errorf("proposal %d = %#v, want %#v", repetition, proposal, first)
		}
	}
}
