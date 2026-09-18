// ARM64ExecutionCapability authorizes one measured ARM64 native profile.
// It binds all artifacts and tools before a request can enter execution.
package isla

import "github.com/HyperMarble/hyperray/machine/arm64"

// ARM64ExecutionCapability is a caller-pinned native execution identity.
type ARM64ExecutionCapability struct {
	manifest         Artifact
	id               string
	profile          string
	executionProfile string
	memoryProfile    string
	memoryConfig     string
	effectiveState   string
	pcRegister       string
	architecture     Artifact
	configuration    Artifact
	memoryModel      Artifact
	solver           ToolIdentity
	semantic         ToolIdentity
	footprint        ToolIdentity
}

// NewARM64ExecutionCapability validates a trusted measured release manifest.
func NewARM64ExecutionCapability(manifest Artifact, verifier Verifier, footprints FootprintEngine, architecture Artifact, configuration Artifact, memoryModel Artifact) (ARM64ExecutionCapability, error) {
	values, err := readARM64ExecutionManifest(manifest.path)
	if err != nil {
		return ARM64ExecutionCapability{}, err
	}
	capability := ARM64ExecutionCapability{manifest: manifest, id: values.CapabilityID, profile: values.Profile, executionProfile: values.ExecutionProfile, memoryProfile: values.MemoryProfile, memoryConfig: values.MemoryConfigDigest, effectiveState: values.EffectiveStateDigest, pcRegister: values.PCRegister, architecture: architecture, configuration: configuration, memoryModel: memoryModel, solver: verifier.solver.identity, semantic: verifier.semantics.identity, footprint: footprints.identity}
	if err := capability.current(); err != nil {
		return ARM64ExecutionCapability{}, err
	}
	if err := values.match(capability); err != nil {
		return ARM64ExecutionCapability{}, err
	}
	return capability, nil
}

func (capability ARM64ExecutionCapability) current() error {
	if err := capability.manifest.current(); err != nil {
		return err
	}
	for _, artifact := range []Artifact{capability.architecture, capability.configuration, capability.memoryModel} {
		if err := artifact.current(); err != nil {
			return err
		}
	}
	for _, tool := range []ToolIdentity{capability.solver, capability.semantic, capability.footprint} {
		if err := toolCurrent(tool); err != nil {
			return err
		}
	}
	return nil
}

func (capability ARM64ExecutionCapability) validateProgram(program Program) error {
	if program.profile != arm64.ProfileName || capability.profile != arm64.ProfileName {
		return engineError(UnsupportedProfile, "ARM execution profile", "format profile differs")
	}
	if capability.executionProfile == arm64ExecutionProfile && (program.memoryProfile != ARM64FetchOnlyV1 || program.memoryInput != nil) {
		return engineError(UnsupportedProfile, "ARM execution profile", "fetch-only identity differs")
	}
	if capability.executionProfile == arm64NormalExecutionProfile && (program.memoryProfile != ARM64NormalS1Fixed4KSIMD || capability.memoryProfile != arm64NormalExecutionProfile || !validDigest(capability.memoryConfig) || !validDigest(capability.effectiveState) || program.memoryIdentity == "") {
		return engineError(UnsupportedProfile, "ARM execution profile", "normal-memory identity differs")
	}
	return nil
}

func (capability ARM64ExecutionCapability) validateRequest(request VerificationRequest) error {
	if request.query.architecture.digest != capability.architecture.digest || request.query.configuration.digest != capability.configuration.digest || request.query.memoryModel.digest != capability.memoryModel.digest {
		return engineError(CoverageMismatch, "ARM execution request", "artifact identity differs from capability")
	}
	if request.query.program.digest == "" {
		return engineError(InvalidInput, "ARM execution request", "program artifact is empty")
	}
	return nil
}
