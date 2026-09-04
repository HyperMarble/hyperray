// Report construction records shared evidence once for the full inventory.
// Instruction records retain their separate output identities.
package isla

func newFootprintReport(engine FootprintEngine, request FootprintRequest, traces []InstructionTrace) FootprintReport {
	evidence := FootprintEvidence{
		Tool: engine.identity, ArchitectureDigest: request.architecture.digest,
		ConfigurationDigest: request.configuration.digest,
		ThreadLimit:         request.threadLimit, TimeLimitSeconds: request.timeLimit,
		MaximumOutputBytes: request.maximumOutputSize,
	}
	return FootprintReport{Instructions: traces, Evidence: evidence}
}
