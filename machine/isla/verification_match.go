// Result acceptance requires both outputs to name the same exact inputs.
// A mismatch returns no result at the final decision boundary.
package isla

func verifiedResult(request VerificationRequest, semantics SemanticReport, proposal Proposal) (VerificationResult, error) {
	semanticEvidence := semantics.Evidence
	solverEvidence := proposal.Evidence
	if semanticEvidence.Tool.Version != solverEvidence.Tool.Version {
		return VerificationResult{}, engineError(ReleaseMismatch, "Isla evidence", "versions differ")
	}
	if semanticEvidence.ArchitectureDigest != solverEvidence.ArchitectureDigest {
		return VerificationResult{}, engineError(CoverageMismatch, "architecture", "semantic and solver digests differ")
	}
	if semanticEvidence.ConfigurationDigest != solverEvidence.ConfigurationDigest {
		return VerificationResult{}, engineError(CoverageMismatch, "configuration", "semantic and solver digests differ")
	}
	if semanticEvidence.ProgramDigest != solverEvidence.ProgramDigest {
		return VerificationResult{}, engineError(CoverageMismatch, "program", "semantic and solver digests differ")
	}
	if semanticEvidence.ProgramDigest != request.query.program.digest {
		return VerificationResult{}, engineError(CoverageMismatch, "program", "request and result digests differ")
	}
	if semanticEvidence.PCVisitLimit != request.query.pcVisitLimit || solverEvidence.PCVisitLimit != request.query.pcVisitLimit {
		return VerificationResult{}, engineError(CoverageMismatch, "instruction visit limit", "request and result limits differ")
	}
	if !semantics.Complete {
		return VerificationResult{}, engineError(CoverageMismatch, "semantic report", "incomplete")
	}
	calls, err := counterexampleCalls(request.query.forbiddenModelCalls, proposal)
	if err != nil {
		return VerificationResult{}, err
	}
	result := acceptedVerificationResult(semantics, proposal)
	result.ModelCalls = calls
	return result, nil
}
