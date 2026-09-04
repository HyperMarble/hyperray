// Failure tests make tool and protocol errors observable through stable codes.
// They must not accept partial command output as a proposal.
package isla_test

import (
	"context"
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestOperationErrorsCannotReturnProposal(t *testing.T) {
	cases := []struct {
		program string
		code    isla.ErrorCode
	}{
		{program: "process-error", code: isla.ProcessFail},
		{program: "tool-error", code: isla.ResultError},
		{program: "visit-limit", code: isla.ResultError},
		{program: "malformed", code: isla.ProtocolError},
	}
	for index := range cases {
		testCase := cases[index]
		t.Run(testCase.program, func(t *testing.T) {
			proposal, err := testEngine(t).Propose(t.Context(), testRequest(t, testCase.program))
			assertProposalError(t, proposal, err, testCase.code)
		})
	}
}

func TestCanceledContextCannotReturnProposal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	proposal, err := testEngine(t).Propose(ctx, testRequest(t, "proof"))
	assertProposalError(t, proposal, err, isla.ResourceLimit)
}

func assertProposalError(t *testing.T, proposal isla.Proposal, err error, code isla.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("proposal = %#v, error = nil", proposal)
	}
	assertErrorCode(t, err, code)
}

func assertErrorCode(t *testing.T, err error, code isla.ErrorCode) {
	t.Helper()
	var failure *isla.Error
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v", err)
	}
	if failure.Code != code {
		t.Errorf("error code = %q, want %q", failure.Code, code)
	}
}
