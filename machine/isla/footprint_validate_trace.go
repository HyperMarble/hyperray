// Trace inventory validation rejects unproved and duplicate engine records.
package isla

func traceInventory(traces []InstructionTrace) (map[uint64]InstructionTrace, error) {
	if len(traces) == 0 {
		return nil, engineError(CoverageMismatch, "footprint inventory", "empty trace inventory")
	}
	result := make(map[uint64]InstructionTrace, len(traces))
	for index := range traces {
		trace := traces[index]
		if trace.TraceCount == 0 {
			return nil, footprintCoverageError("unproved", trace.Address)
		}
		if !validDigest(trace.OutputDigest) {
			return nil, footprintCoverageError("invalid digest for", trace.Address)
		}
		if _, exists := result[trace.Address]; exists {
			return nil, footprintCoverageError("duplicate", trace.Address)
		}
		result[trace.Address] = trace
	}
	return result, nil
}

func validateFootprintEvidence(evidence FootprintEvidence) error {
	if evidence.ReleaseID == "" || evidence.Tool.Path == "" || evidence.Tool.Version == "" {
		return engineError(CoverageMismatch, "footprint evidence", "empty identity")
	}
	digests := []string{
		evidence.Tool.Digest, evidence.ManifestDigest,
		evidence.ArchitectureDigest, evidence.ConfigurationDigest,
	}
	for index := range digests {
		if !validDigest(digests[index]) {
			return engineError(CoverageMismatch, "footprint evidence", "invalid SHA-256 digest")
		}
	}
	if evidence.ThreadLimit == 0 || evidence.TimeLimitSeconds == 0 || evidence.MaximumOutputBytes == 0 {
		return engineError(CoverageMismatch, "footprint evidence", "invalid limit")
	}
	return nil
}
