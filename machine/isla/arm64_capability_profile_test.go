// ARM64 capability tests reject memory-profile substitution.
// A fetch-only capability must never authorize normal-memory input.
package isla

import "testing"

func TestARM64FetchOnlyCapabilityRejectsNormalMemoryProgram(t *testing.T) {
	capability := ARM64ExecutionCapability{profile: "static-little-endian-arm64-macos-macho", executionProfile: arm64ExecutionProfile}
	program := Program{profile: capability.profile, memoryProfile: ARM64NormalS1Fixed4KSIMD, memoryIdentity: "typed"}
	if err := capability.validateProgram(program); err == nil {
		t.Fatal("fetch-only capability accepted normal-memory program")
	}
}
