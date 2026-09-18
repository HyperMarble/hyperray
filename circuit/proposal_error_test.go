// Proposal-error support uses a real process fixture through the public API.
// It never calls private parser functions to create public results.
package circuit_test

import (
	"context"
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func proposalErrorForMode(t *testing.T, mode string) error {
	t.Helper()
	t.Setenv("HYPERRAY_Z3_FIXTURE_MODE", mode)
	tool, err := circuit.NewDifferenceEngine(fixtureToolPath(t))
	if err != nil {
		t.Fatalf("NewDifferenceEngine(fixture) error = %v", err)
	}
	fixture := fixtureWithStatus(t, string(circuit.DifferenceFound))
	reference := balanceRelation(t, fixture.Width, fixture.ReferenceComparison)
	candidate := balanceRelation(t, fixture.Width, fixture.CandidateComparison)
	miter, err := circuit.BuildMiter(reference, candidate)
	if err != nil {
		t.Fatalf("BuildMiter() error = %v", err)
	}
	_, err = tool.Propose(context.Background(), miter)
	return err
}

func TestProposalRejectsInvalidPublicInputs(t *testing.T) {
	fixture := fixtureWithStatus(t, string(circuit.DifferenceFound))
	reference := balanceRelation(t, fixture.Width, fixture.ReferenceComparison)
	candidate := balanceRelation(t, fixture.Width, fixture.CandidateComparison)
	miter, err := circuit.BuildMiter(reference, candidate)
	if err != nil {
		t.Fatalf("BuildMiter() error = %v", err)
	}
	_, err = identifiedEngine(t).Propose(nil, miter)
	requireEngineError(t, err, "nil_context", "proposal")
	_, err = (circuit.DifferenceEngine{}).Propose(context.Background(), miter)
	requireEngineError(t, err, "unidentified_tool", "z3")
	_, err = identifiedEngine(t).Propose(context.Background(), circuit.Miter{})
	requireEngineError(t, err, "empty_miter", "miter")
}
