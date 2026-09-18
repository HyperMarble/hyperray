// Boundary validation requires complete candidate/thread terminal coverage.
// A partial or mismatched set must never produce a final result.
package isla

import "testing"

func TestValidateTerminalEvidenceRequiresCompleteCoverage(t *testing.T) {
	program := Program{returnAddress: 0x100008000, threadEntries: []ThreadEntry{{}, {}}}
	result := VerificationResult{CandidateCount: 2, TerminalEvidence: []TerminalEvidence{
		{CandidateIndex: 0, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: 0x100008000},
		{CandidateIndex: 0, ThreadIndex: 1, Kind: BoundaryReached, DeclaredAddress: 0x100008000},
		{CandidateIndex: 1, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: 0x100008000},
		{CandidateIndex: 1, ThreadIndex: 1, Kind: BoundaryReached, DeclaredAddress: 0x100008000},
	}}
	if err := validateTerminalEvidence(program, result); err != nil {
		t.Fatalf("validateTerminalEvidence() error = %v", err)
	}
	result.TerminalEvidence = result.TerminalEvidence[:3]
	if err := validateTerminalEvidence(program, result); err == nil {
		t.Fatal("validateTerminalEvidence() accepted incomplete coverage")
	}
}

func TestValidateTerminalEvidenceRejectsMismatchAndUnexpectedRVRecords(t *testing.T) {
	arm := Program{returnAddress: 0x100008000, threadEntries: []ThreadEntry{{}}}
	cases := []VerificationResult{
		{CandidateCount: 1, TerminalEvidence: []TerminalEvidence{{CandidateIndex: 1, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: arm.returnAddress}}},
		{CandidateCount: 1, TerminalEvidence: []TerminalEvidence{{CandidateIndex: 0, ThreadIndex: 1, Kind: BoundaryReached, DeclaredAddress: arm.returnAddress}}},
		{CandidateCount: 1, TerminalEvidence: []TerminalEvidence{{CandidateIndex: 0, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: 0x100008004}}},
		{CandidateCount: 1, TerminalEvidence: []TerminalEvidence{{CandidateIndex: 0, ThreadIndex: 0, Kind: TerminalKind("exit"), DeclaredAddress: arm.returnAddress}}},
	}
	for index, result := range cases {
		if err := validateTerminalEvidence(arm, result); err == nil {
			t.Errorf("case %d accepted mismatched evidence", index)
		}
	}
	rv := Program{}
	if err := validateTerminalEvidence(rv, VerificationResult{CandidateCount: 1, TerminalEvidence: []TerminalEvidence{{CandidateIndex: 0, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: 1}}}); err == nil {
		t.Fatal("RV validation accepted unexpected terminal records")
	}
}

func TestAcceptedVerificationResultCopiesTerminalEvidence(t *testing.T) {
	proposal := Proposal{TerminalEvidence: []TerminalEvidence{{CandidateIndex: 0, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: 1}}}
	result := acceptedVerificationResult(SemanticReport{}, proposal)
	proposal.TerminalEvidence[0].DeclaredAddress = 2
	if result.TerminalEvidence[0].DeclaredAddress != 1 {
		t.Fatal("accepted result shares proposal terminal evidence storage")
	}
}

func TestValidateTerminalEvidenceRejectsCardinalityOverflow(t *testing.T) {
	program := Program{returnAddress: 1, threadEntries: []ThreadEntry{{}, {}}}
	result := VerificationResult{CandidateCount: ^uint64(0)}
	if err := validateTerminalEvidence(program, result); err == nil {
		t.Fatal("validateTerminalEvidence() accepted overflowing candidate/thread cardinality")
	}
}

func TestValidateTerminalEvidenceRejectsDuplicatePairs(t *testing.T) {
	program := Program{returnAddress: 1, threadEntries: []ThreadEntry{{}, {}}}
	result := VerificationResult{CandidateCount: 2, TerminalEvidence: []TerminalEvidence{
		{CandidateIndex: 0, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: 1},
		{CandidateIndex: 0, ThreadIndex: 1, Kind: BoundaryReached, DeclaredAddress: 1},
		{CandidateIndex: 0, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: 1},
		{CandidateIndex: 1, ThreadIndex: 1, Kind: BoundaryReached, DeclaredAddress: 1},
	}}
	if err := validateTerminalEvidence(program, result); err == nil {
		t.Fatal("validateTerminalEvidence() accepted duplicate pair with missing pair")
	}
}
