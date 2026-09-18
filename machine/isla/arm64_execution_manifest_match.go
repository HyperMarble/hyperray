// ARM capability matching compares measured identities with caller trust input.
// A matching filename or environment label is never sufficient.
package isla

func (manifest arm64ExecutionManifest) match(capability ARM64ExecutionCapability) error {
	if manifest.ArchitectureDigest != capability.architecture.digest || manifest.ConfigurationDigest != capability.configuration.digest || manifest.MemoryModelDigest != capability.memoryModel.digest {
		return releaseError("ARM artifact identity differs")
	}
	identities := []struct {
		version, digest string
		measured        ToolIdentity
	}{
		{manifest.SolverVersion, manifest.SolverDigest, capability.solver},
		{manifest.SemanticVersion, manifest.SemanticDigest, capability.semantic},
		{manifest.FootprintVersion, manifest.FootprintDigest, capability.footprint},
	}
	for _, identity := range identities {
		if identity.version != identity.measured.Version || identity.digest != identity.measured.Digest {
			return releaseError("ARM tool identity differs")
		}
	}
	return nil
}

func (capability ARM64ExecutionCapability) matchesRelease(release FootprintRelease) error {
	if release.tool != capability.footprint || release.architecture.digest != capability.architecture.digest || release.configuration.digest != capability.configuration.digest {
		return releaseError("ARM capability and footprint release identities differ")
	}
	return nil
}

func (capability ARM64ExecutionCapability) matchesVerifier(verifier Verifier, footprints FootprintEngine) error {
	if verifier.solver.identity != capability.solver || verifier.semantics.identity != capability.semantic || footprints.identity != capability.footprint {
		return releaseError("ARM capability and verifier tool identities differ")
	}
	return nil
}
