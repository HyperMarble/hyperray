// The semantic report and the solver proposal are taken from the same
// request and compared afterwards, so neither waits for the other.
package isla

import (
	"context"
	"sync"
)

func inspectAndPropose(ctx context.Context, verifier Verifier, request VerificationRequest) (SemanticReport, Proposal, error) {
	var semantics SemanticReport
	var proposal Proposal
	var semanticErr, proposalErr error

	var both sync.WaitGroup
	both.Add(2)
	go func() {
		defer both.Done()
		semantics, semanticErr = verifier.semantics.inspect(ctx, request)
	}()
	go func() {
		defer both.Done()
		proposal, proposalErr = verifier.solver.Propose(ctx, request.query)
	}()
	both.Wait()

	if semanticErr != nil {
		return SemanticReport{}, Proposal{}, semanticErr
	}
	if proposalErr != nil {
		return SemanticReport{}, Proposal{}, proposalErr
	}
	return semantics, proposal, nil
}
