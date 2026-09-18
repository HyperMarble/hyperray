// ARM capability metadata exposes only immutable scalar identity values.
// Callers cannot mutate the trusted artifact or tool bindings.
package isla

func (capability ARM64ExecutionCapability) PCRegister() string { return capability.pcRegister }
func (capability ARM64ExecutionCapability) ExecutionProfile() string {
	return capability.executionProfile
}
func (capability ARM64ExecutionCapability) CapabilityID() string  { return capability.id }
func (capability ARM64ExecutionCapability) MemoryProfile() string { return capability.memoryProfile }
func (capability ARM64ExecutionCapability) MemoryConfigurationDigest() string {
	return capability.memoryConfig
}
func (capability ARM64ExecutionCapability) EffectiveStateDigest() string {
	return capability.effectiveState
}

func (capability ARM64ExecutionCapability) evidence() *ARM64CapabilityEvidence {
	return &ARM64CapabilityEvidence{
		CapabilityID: capability.id, ExecutionProfile: capability.executionProfile,
		MemoryProfile: capability.memoryProfile, MemoryConfigurationDigest: capability.memoryConfig,
		EffectiveStateDigest: capability.effectiveState,
	}
}
