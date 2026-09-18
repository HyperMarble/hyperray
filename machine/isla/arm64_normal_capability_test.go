// ARM64 normal-memory manifest tests require measured profile artifacts.
// They must reject a format-only manifest or missing native state identity.
package isla

import (
	"strings"
	"testing"
)

func TestARM64NormalMemoryManifestRequiresMeasuredIdentity(t *testing.T) {
	manifest := validARM64ExecutionManifest()
	manifest.ExecutionProfile = arm64NormalExecutionProfile
	manifest.MemoryProfile = arm64NormalExecutionProfile
	manifest.MemoryConfigDigest = strings.Repeat("1", 64)
	manifest.EffectiveStateDigest = strings.Repeat("2", 64)
	if err := manifest.valid(); err != nil {
		t.Fatalf("normal-memory manifest rejected: %v", err)
	}
	for name, mutate := range map[string]func(*arm64ExecutionManifest){
		"profile": func(value *arm64ExecutionManifest) { value.MemoryProfile = arm64ExecutionProfile },
		"config":  func(value *arm64ExecutionManifest) { value.MemoryConfigDigest = "" },
		"state":   func(value *arm64ExecutionManifest) { value.EffectiveStateDigest = "bad" },
	} {
		invalid := manifest
		mutate(&invalid)
		if err := invalid.valid(); err == nil {
			t.Errorf("normal-memory manifest accepted invalid %s identity", name)
		}
	}
}
