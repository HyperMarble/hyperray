// Semantic report construction copies parser results into public evidence.
// It marks coverage complete only after all parser checks pass.
package isla

func newSemanticReport(engine SemanticEngine, request VerificationRequest, output commandOutput, summary semanticSummary, dispositions []DiagnosticDisposition) SemanticReport {
	evidence := SemanticEvidence{
		Tool: engine.identity, ArchitectureDigest: request.query.architecture.digest,
		ConfigurationDigest: request.query.configuration.digest,
		ProgramDigest:       request.query.program.digest, OutputDigest: rawOutputDigest(output),
		ThreadLimit: request.threadLimit, TimeLimitSeconds: request.query.timeLimit,
		PCVisitLimit:  request.query.pcVisitLimit,
		MemoryLimitMB: request.memoryLimitMB, MaximumOutputBytes: request.query.maximumOutputSize,
		ElapsedMilliseconds: output.elapsed.Milliseconds(),
	}
	return SemanticReport{
		Complete: true, ThreadCount: summary.threadCount, TraceCount: summary.traceCount,
		InstructionEventCount: summary.instructionEventCount,
		InstructionEncodings:  summary.instructionEncodings,
		Instructions:          summary.instructions,
		Threads:               copySemanticThreads(summary.threads),
		FootprintEncodings:    summary.footprintEncodings,
		FinalAssertion:        summary.finalAssertion, TraceOutput: output.stdout,
		Diagnostics: output.diagnostics, Dispositions: dispositions, Evidence: evidence,
	}
}
