// Verification results combine complete semantic evidence with one solver result.
// A value exists only after the identity and coverage checks pass.
package isla

// VerificationStatus identifies the final result for the bounded Isla query.
type VerificationStatus string

const (
	// Proved means the solver found no counterexample in the complete accepted model.
	Proved VerificationStatus = "PROVED"
	// Disproved means the solver returned an allowed counterexample.
	Disproved VerificationStatus = "DISPROVED"
)

// VerificationResult is the accepted same-program Isla result.
type VerificationResult struct {
	Status              VerificationStatus `json:"status"`
	QueryName           string             `json:"query_name"`
	CandidateCount      uint64             `json:"candidate_count"`
	CounterexampleCount uint64             `json:"counterexample_count"`
	CounterexampleState string             `json:"counterexample_state,omitempty"`
	TerminalEvidence    []TerminalEvidence `json:"terminal_evidence,omitempty"`
	ModelCalls          map[string]bool    `json:"model_calls,omitempty"`
	Semantics           SemanticReport     `json:"semantics"`
	SolverEvidence      Evidence           `json:"solver_evidence"`
}
