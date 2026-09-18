// Semantic failure tests reject incomplete or malformed coverage evidence.
// No tested failure may return a verification verdict.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestVerifierRejectsSemanticCoverageFailures(t *testing.T) {
	cases := []struct {
		program string
		code    isla.ErrorCode
	}{
		{program: "semantic-missing", code: isla.CoverageMismatch},
		{program: "semantic-extra", code: isla.CoverageMismatch},
		{program: "semantic-duplicate", code: isla.ProtocolError},
		{program: "semantic-malformed", code: isla.ProtocolError},
		{program: "semantic-warning", code: isla.ProtocolError},
		{program: "semantic-process-error", code: isla.ProcessFail},
		{program: "semantic-change-program", code: isla.ArtifactChanged},
	}
	verifier := testVerifier(t)
	for index := range cases {
		testCase := cases[index]
		t.Run(testCase.program, func(t *testing.T) {
			result, err := verifier.Verify(t.Context(), verificationRequest(t, testCase.program, 4096))
			assertVerificationError(t, result, err, testCase.code)
		})
	}
}

func TestVerifierRejectsSemanticOutputLimit(t *testing.T) {
	result, err := testVerifier(t).Verify(t.Context(), verificationRequest(t, "proof", 20))
	assertVerificationError(t, result, err, isla.ResourceLimit)
}

func TestVerifierRejectsNilContext(t *testing.T) {
	result, err := testVerifier(t).Verify(nil, verificationRequest(t, "proof", 4096))
	assertVerificationError(t, result, err, isla.InvalidInput)
}

func TestVerifierRejectsSolverFailures(t *testing.T) {
	cases := []struct {
		program string
		code    isla.ErrorCode
	}{
		{program: "tool-error", code: isla.ResultError},
		{program: "malformed", code: isla.ProtocolError},
		{program: "process-error", code: isla.ProcessFail},
		{program: "proposal-warning", code: isla.ProtocolError},
	}
	for index := range cases {
		testCase := cases[index]
		result, err := testVerifier(t).Verify(t.Context(), verificationRequest(t, testCase.program, 4096))
		assertVerificationError(t, result, err, testCase.code)
	}
}
