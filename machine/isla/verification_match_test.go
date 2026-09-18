// Matching tests reject each possible mismatch at the final verdict boundary.
// They exercise defensive checks that public constructors preserve.
package isla

import (
	"errors"
	"testing"
)

func matchingFixture() (VerificationRequest, SemanticReport, Proposal) {
	evidence := SemanticEvidence{
		Tool: ToolIdentity{Version: "v1"}, ArchitectureDigest: "architecture",
		ConfigurationDigest: "configuration", ProgramDigest: "program",
	}
	request := VerificationRequest{query: Request{program: Artifact{digest: "program"}}}
	proposal := Proposal{Evidence: Evidence{
		Tool: ToolIdentity{Version: "v1"}, ArchitectureDigest: "architecture",
		ConfigurationDigest: "configuration", ProgramDigest: "program",
	}}
	return request, SemanticReport{Complete: true, Evidence: evidence}, proposal
}

func requireMatchingFailure(t *testing.T, request VerificationRequest, semantics SemanticReport, proposal Proposal, code ErrorCode) {
	t.Helper()
	result, err := verifiedResult(request, semantics, proposal)
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Errorf("verifiedResult() error = %v, want %q", err, code)
	}
	if result.Status != "" {
		t.Errorf("verifiedResult() result = %#v", result)
	}
}

func TestMatchingRejectsToolAndModelDifferences(t *testing.T) {
	request, semantics, proposal := matchingFixture()
	proposal.Evidence.Tool.Version = "v2"
	requireMatchingFailure(t, request, semantics, proposal, ReleaseMismatch)
	request, semantics, proposal = matchingFixture()
	proposal.Evidence.ArchitectureDigest = "other"
	requireMatchingFailure(t, request, semantics, proposal, CoverageMismatch)
	request, semantics, proposal = matchingFixture()
	proposal.Evidence.ConfigurationDigest = "other"
	requireMatchingFailure(t, request, semantics, proposal, CoverageMismatch)
}

func TestMatchingRejectsProgramAndCompletionDifferences(t *testing.T) {
	request, semantics, proposal := matchingFixture()
	proposal.Evidence.ProgramDigest = "other"
	requireMatchingFailure(t, request, semantics, proposal, CoverageMismatch)
	request, semantics, proposal = matchingFixture()
	request.query.program.digest = "other"
	requireMatchingFailure(t, request, semantics, proposal, CoverageMismatch)
	request, semantics, proposal = matchingFixture()
	semantics.Complete = false
	requireMatchingFailure(t, request, semantics, proposal, CoverageMismatch)
}
