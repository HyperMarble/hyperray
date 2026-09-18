// Boundary validation requires one terminal record per candidate and thread.
// It never accepts a partial proof or counterexample.
package isla

func validateTerminalEvidence(program Program, result VerificationResult) error {
	if program.returnAddress == 0 {
		if len(result.TerminalEvidence) != 0 {
			return engineError(ProtocolError, "terminal evidence", "record present without a declared boundary")
		}
		return nil
	}
	if result.CandidateCount == 0 || len(program.threadEntries) == 0 {
		return engineError(ProtocolError, "terminal evidence", "boundary result has no candidates or threads")
	}
	threadCount := program.ThreadCount()
	if result.CandidateCount > ^uint64(0)/threadCount {
		return engineError(ProtocolError, "terminal evidence", "candidate/thread count overflows")
	}
	if uint64(len(result.TerminalEvidence)) != result.CandidateCount*threadCount {
		return engineError(ProtocolError, "terminal evidence", "candidate/thread coverage is incomplete")
	}
	seen := make(map[string]struct{}, len(result.TerminalEvidence))
	for _, record := range result.TerminalEvidence {
		if record.CandidateIndex >= result.CandidateCount || record.ThreadIndex >= threadCount {
			return engineError(ProtocolError, "terminal evidence", "candidate or thread index is outside the result")
		}
		if record.Kind != BoundaryReached || record.DeclaredAddress != program.returnAddress {
			return engineError(ProtocolError, "terminal evidence", "record does not match the declared boundary")
		}
		key := terminalRecordKey(record)
		if _, exists := seen[key]; exists {
			return engineError(ProtocolError, "terminal evidence", "duplicate candidate/thread record")
		}
		seen[key] = struct{}{}
	}
	return nil
}
