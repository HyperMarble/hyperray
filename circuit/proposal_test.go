// These tests measure real engine proposals for both balance fixtures.
// They never call raw UNSAT a proof or a coverage certificate.
package circuit_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestBalanceMutationDifference(t *testing.T) {
	fixture := fixtureWithStatus(t, string(circuit.DifferenceFound))
	proposal := proposeFixture(t, fixture)
	wantAssignments := []circuit.Assignment{
		{Role: circuit.StateAssignment, Name: "balance", Value: fixture.ExpectedBalance},
		{Role: circuit.InputAssignment, Name: "cost", Value: fixture.ExpectedCost},
	}
	if proposal.Status != circuit.DifferenceFound || proposal.Difference == nil {
		t.Fatalf("proposal = %#v, want a concrete difference", proposal)
	}
	difference := proposal.Difference
	if !reflect.DeepEqual(difference.Assignments, wantAssignments) {
		t.Errorf("assignments = %#v, want %#v", difference.Assignments, wantAssignments)
	}
	if difference.Kind != circuit.ObservationOutput || difference.Name != "insufficient" {
		t.Errorf("different output = %q %q", difference.Kind, difference.Name)
	}
	if difference.ReferenceValue != fixture.ExpectedReference {
		t.Errorf("reference value = %q, want %q", difference.ReferenceValue, fixture.ExpectedReference)
	}
	if difference.CandidateValue != fixture.ExpectedCandidate {
		t.Errorf("candidate value = %q, want %q", difference.CandidateValue, fixture.ExpectedCandidate)
	}
}

func TestEqualBalanceRelationIsUnvalidatedUnsat(t *testing.T) {
	fixture := fixtureWithStatus(t, string(circuit.UnvalidatedUnsat))
	proposal := proposeFixture(t, fixture)
	if proposal.Status != circuit.UnvalidatedUnsat {
		t.Errorf("Proposal.Status = %q, want %q", proposal.Status, circuit.UnvalidatedUnsat)
	}
	if proposal.Difference != nil {
		t.Errorf("Proposal.Difference = %#v, want nil", proposal.Difference)
	}
}

func proposeFixture(t *testing.T, fixture balanceFixture) circuit.Proposal {
	t.Helper()
	reference := balanceRelation(t, fixture.Width, fixture.ReferenceComparison)
	candidate := balanceRelation(t, fixture.Width, fixture.CandidateComparison)
	miter, err := circuit.BuildMiter(reference, candidate)
	if err != nil {
		t.Fatalf("BuildMiter() error = %v", err)
	}
	tool := identifiedEngine(t)
	proposal, err := tool.Propose(context.Background(), miter)
	if err != nil {
		t.Fatalf("DifferenceEngine.Propose() error = %v", err)
	}
	if proposal.Tool != tool.Identity() || proposal.MiterDigest != miter.Digest() {
		t.Errorf("proposal identities do not match the inputs")
	}
	return proposal
}
