// Result tests require safe and unsafe programs to use one public verifier.
// Each result must expose matching same-program semantic evidence.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicVerifierReturnsBothResults(t *testing.T) {
	verifier := testVerifier(t)
	proof, err := verifier.Verify(t.Context(), verificationRequest(t, "proof", 4096))
	if err != nil {
		t.Fatalf("proof Verify() error = %v", err)
	}
	if proof.Status != isla.Proved || !proof.Semantics.Complete {
		t.Errorf("proof = %#v", proof)
	}
	if proof.Semantics.Evidence.ProgramDigest != proof.SolverEvidence.ProgramDigest {
		t.Errorf("proof program digests differ")
	}
	counterexample, err := verifier.Verify(t.Context(), verificationRequest(t, "counterexample", 4096))
	if err != nil {
		t.Fatalf("counterexample Verify() error = %v", err)
	}
	if counterexample.Status != isla.Disproved || counterexample.CounterexampleState == "" {
		t.Errorf("counterexample = %#v", counterexample)
	}
	if counterexample.Semantics.InstructionEventCount != 1 {
		t.Errorf("semantic report = %#v", counterexample.Semantics)
	}
}

func TestVerificationRequestRejectsInvalidLimits(t *testing.T) {
	request, err := isla.NewVerificationRequest(testRequest(t, "proof"), 0, 1)
	if err == nil {
		t.Errorf("NewVerificationRequest() = %#v, nil error", request)
	}
	request, err = isla.NewVerificationRequest(isla.Request{}, 1, 1)
	if err == nil {
		t.Errorf("NewVerificationRequest() with empty query = %#v, nil error", request)
	}
}
