//go:build isla_integration

// Real verification tests join the pinned semantic dump and solver results.
// Both programs must pass through the same public verifier.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealIslaSameProgramResults(t *testing.T) {
	solver, err := isla.NewEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA"))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	semantics, err := isla.NewSemanticEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA_DUMP"))
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	verifier, err := isla.NewVerifier(solver, semantics)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	proof := realVerification(t, verifier, "addi-proof.toml")
	if proof.Status != isla.Proved || !proof.Semantics.Complete {
		t.Errorf("proof = %#v", proof)
	}
	counterexample := realVerification(t, verifier, "addi-counterexample.toml")
	if counterexample.Status != isla.Disproved {
		t.Errorf("counterexample = %#v", counterexample)
	}
	wantState := "0:x5=#x0000000000000003;"
	if counterexample.CounterexampleState != wantState {
		t.Errorf("CounterexampleState = %q, want %q", counterexample.CounterexampleState, wantState)
	}
	t.Logf("proof=%s counterexample=%s instructions=%d", proof.Status, counterexample.Status, proof.Semantics.InstructionEventCount)
}

func realVerification(t *testing.T, verifier isla.Verifier, fixture string) isla.VerificationResult {
	t.Helper()
	request, err := isla.NewVerificationRequest(realRequest(t, fixture), 2, 2048)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	result, err := verifier.Verify(t.Context(), request)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	return result
}
