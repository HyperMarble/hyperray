//go:build isla_integration

// Real Isla tests measure correct and incorrect claims through the public API.
// They must not replace the engine with a fixture process.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealIslaReturnsBothResults(t *testing.T) {
	engine, err := isla.NewEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA"))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	proofRequest := realRequest(t, "addi-proof.toml")
	proof := realProposal(t, engine, proofRequest)
	if proof.Status != isla.NoCounterexampleFound || proof.CounterexampleCount != 0 {
		t.Errorf("proof proposal = %#v", proof)
	}
	counterexampleRequest := realRequest(t, "addi-counterexample.toml")
	counterexample := realProposal(t, engine, counterexampleRequest)
	if counterexample.Status != isla.CounterexampleFound || counterexample.CounterexampleCount != 1 {
		t.Errorf("counterexample proposal = %#v", counterexample)
	}
	wantState := "0:x5=#x0000000000000003;"
	if counterexample.CounterexampleState != wantState {
		t.Errorf("CounterexampleState = %q, want %q", counterexample.CounterexampleState, wantState)
	}
	t.Logf("proof=%s counterexample=%s state=%s", proof.Status, counterexample.Status, counterexample.CounterexampleState)
}

func realProposal(t *testing.T, engine isla.Engine, request isla.Request) isla.Proposal {
	t.Helper()
	proposal, err := engine.Propose(t.Context(), request)
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	return proposal
}
