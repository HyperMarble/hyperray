// Proposal construction records the result and every input identity.
// It must keep a counterexample state only for an allowed execution.
package isla

func proposalFromResult(engine Engine, request Request, output commandOutput, result herdResult) Proposal {
	status := NoCounterexampleFound
	if result.counterexamples > 0 {
		status = CounterexampleFound
	}
	evidence := Evidence{
		Tool:                engine.identity,
		ArchitectureDigest:  request.architecture.digest,
		ConfigurationDigest: request.configuration.digest,
		MemoryModelDigest:   request.memoryModel.digest,
		ProgramDigest:       request.program.digest,
		OutputDigest:        rawOutputDigest(output),
		PCVisitLimit:        request.pcVisitLimit,
		TimeLimitSeconds:    request.timeLimit,
		ElapsedMilliseconds: output.elapsed.Milliseconds(),
		Diagnostics:         output.diagnostics,
	}
	return Proposal{
		Status:              status,
		QueryName:           result.name,
		CandidateCount:      result.candidates,
		CounterexampleCount: result.counterexamples,
		CounterexampleState: result.counterexampleState,
		Evidence:            evidence,
	}
}
