// ARM manifests must name the exact first native capability.
// Static Mach-O format identity does not authorize broader memory support.
package isla

import "github.com/HyperMarble/hyperray/machine/arm64"

func (manifest arm64ExecutionManifest) valid() error {
	if manifest.CapabilityID == "" || !validARM64ExecutionProfile(manifest.ExecutionProfile) || manifest.Profile != arm64.ProfileName || manifest.PCRegister != arm64ProgramCounterRegister || manifest.ContinuationBoundaryVersion != arm64ContinuationVersion || manifest.PostResetVersion != arm64PostResetVersion || manifest.TerminalEvidenceVersion != arm64TerminalEvidenceVersion {
		return releaseError("ARM capability protocol identity differs")
	}
	if err := validateARM64MemoryManifest(manifest); err != nil {
		return err
	}
	values := []string{manifest.ArchitectureDigest, manifest.ConfigurationDigest, manifest.MemoryModelDigest, manifest.SolverDigest, manifest.SemanticDigest, manifest.FootprintDigest}
	for _, value := range values {
		if !validDigest(value) {
			return releaseError("ARM capability manifest has an invalid SHA-256 digest")
		}
	}
	if manifest.SolverVersion == "" || manifest.SemanticVersion == "" || manifest.FootprintVersion == "" {
		return releaseError("ARM capability manifest has an empty tool identity")
	}
	return nil
}

func validARM64ExecutionProfile(profile string) bool {
	return profile == arm64ExecutionProfile || profile == arm64NormalExecutionProfile
}

func validateARM64MemoryManifest(manifest arm64ExecutionManifest) error {
	if manifest.ExecutionProfile == arm64ExecutionProfile && manifest.MemoryProfile == "" && manifest.MemoryConfigDigest == "" && manifest.EffectiveStateDigest == "" {
		return nil
	}
	if manifest.ExecutionProfile != arm64NormalExecutionProfile || manifest.MemoryProfile != arm64NormalExecutionProfile || !validDigest(manifest.MemoryConfigDigest) || !validDigest(manifest.EffectiveStateDigest) {
		return releaseError("ARM normal-memory capability identity differs")
	}
	return nil
}
