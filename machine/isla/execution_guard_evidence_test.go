// Result acceptance binds both recorded guards to the caller's request.
// A changed guard must not retain a logic verdict.
package isla

import "testing"

func TestSemanticEvidenceRetainsVisitLimit(t *testing.T) {
	request := VerificationRequest{query: Request{pcVisitLimit: 17}}
	report := newSemanticReport(SemanticEngine{}, request, commandOutput{}, semanticSummary{}, nil)
	if report.Evidence.PCVisitLimit != request.query.pcVisitLimit {
		t.Fatalf("visit evidence = %d", report.Evidence.PCVisitLimit)
	}
}

func TestMatchingRejectsVisitLimitDifferences(t *testing.T) {
	request, semantics, proposal := matchingFixture()
	request.query.pcVisitLimit = 4
	semantics.Evidence.PCVisitLimit = 4
	proposal.Evidence.PCVisitLimit = 4
	result, err := verifiedResult(request, semantics, proposal)
	if err != nil || result.Status == "" {
		t.Fatalf("matching guards failed: result=%#v error=%v", result, err)
	}
	semantics.Evidence.PCVisitLimit = 2
	requireMatchingFailure(t, request, semantics, proposal, CoverageMismatch)
	semantics.Evidence.PCVisitLimit = 4
	proposal.Evidence.PCVisitLimit = 2
	requireMatchingFailure(t, request, semantics, proposal, CoverageMismatch)
	proposal.Evidence.PCVisitLimit = 4
	request.query.pcVisitLimit = 2
	requireMatchingFailure(t, request, semantics, proposal, CoverageMismatch)
}
