// Same-program verification joins coverage evidence and the solver proposal.
// No failure path returns a partial or guessed verdict.
package isla

import "context"

// Verify checks semantic coverage before it returns a final bounded verdict.
func (verifier Verifier) Verify(ctx context.Context, request VerificationRequest) (VerificationResult, error) {
	if ctx == nil {
		return VerificationResult{}, engineError(InvalidInput, "context", "nil")
	}
	if err := verifier.current(); err != nil {
		return VerificationResult{}, err
	}
	if err := request.current(); err != nil {
		return VerificationResult{}, err
	}
	semantics, proposal, err := inspectAndPropose(ctx, verifier, request)
	if err != nil {
		return VerificationResult{}, err
	}
	return verifiedResult(request, semantics, proposal)
}

func (verifier Verifier) current() error {
	if err := verifier.solver.current(); err != nil {
		return err
	}
	if err := verifier.semantics.current(); err != nil {
		return err
	}
	if verifier.solver.identity.Version != verifier.semantics.identity.Version {
		return engineError(ReleaseMismatch, "Isla tools", "versions differ")
	}
	return nil
}

func acceptedVerificationResult(semantics SemanticReport, proposal Proposal) VerificationResult {
	status := Proved
	if proposal.Status == CounterexampleFound {
		status = Disproved
	}
	return VerificationResult{
		Status: status, QueryName: proposal.QueryName,
		CandidateCount:      proposal.CandidateCount,
		CounterexampleCount: proposal.CounterexampleCount,
		CounterexampleState: proposal.CounterexampleState,
		TerminalEvidence:    append([]TerminalEvidence(nil), proposal.TerminalEvidence...),
		Semantics:           semantics, SolverEvidence: proposal.Evidence,
	}
}
