// Validation tests require complete and graph-bound proof questions.
// They never turn malformed requirements into vacuous proofs.
package proof_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/proof"
)

func TestRequirementFailures(t *testing.T) {
	cases := []struct {
		name        string
		requirement proof.Requirement
		code        string
	}{
		{"empty ID", proof.Requirement{Kind: proof.RequirementSafety}, "empty_requirement_id"},
		{"unknown kind", proof.Requirement{ID: "bad", Kind: "other"}, "unknown_requirement_kind"},
		{"unknown state", proof.Requirement{ID: "bad", Kind: proof.RequirementSafety,
			BadStateIDs: []string{"missing"}}, "unknown_identifier"},
		{"duplicate state", proof.Requirement{ID: "bad", Kind: proof.RequirementSafety,
			BadStateIDs: []string{"work", "work"}}, "duplicate_identifier"},
		{"empty state", proof.Requirement{ID: "bad", Kind: proof.RequirementSafety,
			BadStateIDs: []string{""}}, "empty_identifier"},
		{"safety terminal", proof.Requirement{ID: "bad", Kind: proof.RequirementSafety,
			TerminalStateIDs: []string{"work"}}, "unexpected_terminal_state"},
		{"termination safety", proof.Requirement{ID: "bad", Kind: proof.RequirementTotalTermination,
			BadTransitionIDs: []string{"compiled-step"}, TerminalStateIDs: []string{"work"}},
			"unexpected_safety_predicate"},
		{"termination no terminal", proof.Requirement{ID: "bad", Kind: proof.RequirementTotalTermination},
			"empty_terminal_states"},
	}
	validated := validatedFixture(t)
	for _, testCase := range cases {
		query := proof.Query{RootIDs: []string{"main-root"}, Requirement: testCase.requirement}
		_, err := proof.Check(validated, query)
		requireProofError(t, err, testCase.code)
	}
}

func TestUnknownRootIsCoverageError(t *testing.T) {
	_, err := proof.Check(validatedFixture(t), safeQuery("missing-root"))
	failure := requireProofError(t, err, "invalid_coverage")
	if len(failure.References) != 1 {
		t.Errorf("Error.References = %v", failure.References)
	}
}
