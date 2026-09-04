// Report construction records shared evidence once for the full inventory.
// Instruction records retain their separate output identities.
package isla

func newFootprintReport(engine FootprintEngine, request FootprintRequest, traces []InstructionTrace) FootprintReport {
	evidence := FootprintEvidence{
		Tool: engine.identity, ReleaseID: request.release.id,
		ManifestDigest:      request.release.manifest.digest,
		ArchitectureDigest:  request.release.architecture.digest,
		ConfigurationDigest: request.release.configuration.digest,
		ThreadLimit:         request.threadLimit, TimeLimitSeconds: request.timeLimit,
		MaximumOutputBytes: request.maximumOutputSize,
	}
	return FootprintReport{Instructions: traces, Evidence: evidence}
}
