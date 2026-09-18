// ARM execution capability tests reject identity and request substitution.
// A format profile alone must not authorize native execution.
package isla

import (
	"strings"
	"testing"
)

func TestARM64ExecutionManifestRejectsWrongExecutionIdentity(t *testing.T) {
	manifest := validARM64ExecutionManifest()
	cases := []func(*arm64ExecutionManifest){
		func(value *arm64ExecutionManifest) { value.Profile = "static-little-endian-rv64-linux-elf-lp64d" },
		func(value *arm64ExecutionManifest) { value.ExecutionProfile = "" },
		func(value *arm64ExecutionManifest) { value.PCRegister = "PC" },
		func(value *arm64ExecutionManifest) { value.TerminalEvidenceVersion = "v2" },
		func(value *arm64ExecutionManifest) { value.SolverDigest = "bad" },
	}
	for index, change := range cases {
		value := manifest
		change(&value)
		if err := value.valid(); err == nil {
			t.Errorf("case %d accepted invalid ARM capability manifest", index)
		}
	}
}

func TestARM64ExecutionManifestAcceptsNativeProtocolIdentity(t *testing.T) {
	if err := validARM64ExecutionManifest().valid(); err != nil {
		t.Fatalf("valid ARM capability manifest rejected: %v", err)
	}
}

func TestARM64ExecutionCapabilityRejectsRequestSubstitution(t *testing.T) {
	capability := ARM64ExecutionCapability{
		architecture:  Artifact{digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		configuration: Artifact{digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		memoryModel:   Artifact{digest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
	}
	request := VerificationRequest{query: Request{
		architecture:  Artifact{digest: capability.architecture.digest},
		configuration: Artifact{digest: capability.configuration.digest},
		memoryModel:   Artifact{digest: capability.memoryModel.digest},
		program:       Artifact{digest: strings.Repeat("d", 64)},
	}}
	if err := capability.validateRequest(request); err != nil {
		t.Fatalf("validateRequest() error = %v", err)
	}
	request.query.configuration.digest = capability.architecture.digest
	if err := capability.validateRequest(request); err == nil {
		t.Fatal("validateRequest() accepted substituted configuration")
	}
}

func TestARM64ExecutionCapabilityBindsEveryRequestArtifact(t *testing.T) {
	capability := ARM64ExecutionCapability{
		architecture:  Artifact{digest: strings.Repeat("a", 64)},
		configuration: Artifact{digest: strings.Repeat("b", 64)},
		memoryModel:   Artifact{digest: strings.Repeat("c", 64)},
	}
	request := VerificationRequest{query: Request{
		architecture:  capability.architecture,
		configuration: capability.configuration,
		memoryModel:   capability.memoryModel,
		program:       Artifact{digest: strings.Repeat("d", 64)},
	}}
	for name, mutate := range map[string]func(*Request){
		"architecture":     func(value *Request) { value.architecture.digest = strings.Repeat("e", 64) },
		"configuration":    func(value *Request) { value.configuration.digest = strings.Repeat("e", 64) },
		"CAT memory model": func(value *Request) { value.memoryModel.digest = strings.Repeat("e", 64) },
	} {
		mutated := request
		mutate(&mutated.query)
		if err := capability.validateRequest(mutated); err == nil {
			t.Errorf("validateRequest() accepted substituted %s", name)
		}
	}
}

func TestARM64ExecutionCapabilityRejectsFootprintReleaseSubstitution(t *testing.T) {
	capability := ARM64ExecutionCapability{
		architecture:  Artifact{digest: strings.Repeat("a", 64)},
		configuration: Artifact{digest: strings.Repeat("b", 64)},
		footprint:     ToolIdentity{Path: "/footprint", Version: "v1", Digest: strings.Repeat("c", 64)},
	}
	release := FootprintRelease{
		architecture:  capability.architecture,
		configuration: capability.configuration,
		tool:          ToolIdentity{Path: "/other-footprint", Version: "v1", Digest: strings.Repeat("d", 64)},
	}
	if err := capability.matchesRelease(release); err == nil {
		t.Fatal("matchesRelease() accepted substituted footprint tool")
	}
}

func TestARM64ExecutionCapabilityRejectsVerifierSubstitution(t *testing.T) {
	capability := ARM64ExecutionCapability{
		solver:    ToolIdentity{Path: "/solver", Version: "v1", Digest: strings.Repeat("a", 64)},
		semantic:  ToolIdentity{Path: "/semantic", Version: "v1", Digest: strings.Repeat("b", 64)},
		footprint: ToolIdentity{Path: "/footprint", Version: "v1", Digest: strings.Repeat("c", 64)},
	}
	verifier := Verifier{
		solver:    Engine{identity: capability.solver},
		semantics: SemanticEngine{identity: ToolIdentity{Path: "/other-semantic", Version: "v1", Digest: strings.Repeat("d", 64)}},
	}
	if err := capability.matchesVerifier(verifier, FootprintEngine{identity: capability.footprint}); err == nil {
		t.Fatal("matchesVerifier() accepted substituted semantic tool")
	}
}

func TestARM64ExecutionManifestMatchesEveryMeasuredTool(t *testing.T) {
	capability := ARM64ExecutionCapability{
		architecture:  Artifact{digest: strings.Repeat("a", 64)},
		configuration: Artifact{digest: strings.Repeat("b", 64)},
		memoryModel:   Artifact{digest: strings.Repeat("c", 64)},
		solver:        ToolIdentity{Version: "solver", Digest: strings.Repeat("d", 64)},
		semantic:      ToolIdentity{Version: "semantic", Digest: strings.Repeat("e", 64)},
		footprint:     ToolIdentity{Version: "footprint", Digest: strings.Repeat("f", 64)},
	}
	manifest := validARM64ExecutionManifest()
	manifest.ArchitectureDigest = capability.architecture.digest
	manifest.ConfigurationDigest = capability.configuration.digest
	manifest.MemoryModelDigest = capability.memoryModel.digest
	manifest.SolverVersion, manifest.SolverDigest = capability.solver.Version, capability.solver.Digest
	manifest.SemanticVersion, manifest.SemanticDigest = capability.semantic.Version, capability.semantic.Digest
	manifest.FootprintVersion, manifest.FootprintDigest = capability.footprint.Version, capability.footprint.Digest
	for name, mutate := range map[string]func(*arm64ExecutionManifest){
		"solver":    func(value *arm64ExecutionManifest) { value.SolverDigest = strings.Repeat("0", 64) },
		"semantic":  func(value *arm64ExecutionManifest) { value.SemanticDigest = strings.Repeat("0", 64) },
		"footprint": func(value *arm64ExecutionManifest) { value.FootprintDigest = strings.Repeat("0", 64) },
	} {
		mutated := manifest
		mutate(&mutated)
		if err := mutated.match(capability); err == nil {
			t.Errorf("manifest.match() accepted substituted %s identity", name)
		}
	}
}

func validARM64ExecutionManifest() arm64ExecutionManifest {
	return arm64ExecutionManifest{
		CapabilityID: "arm64-fetch-only-test", Profile: "static-little-endian-arm64-macos-macho",
		ExecutionProfile: "arm64-fetch-only-v1", PCRegister: "_PC",
		ContinuationBoundaryVersion: "continuation-boundary-v1", PostResetVersion: "arm64-post-reset-v1", TerminalEvidenceVersion: "v1",
		ArchitectureDigest: strings.Repeat("a", 64), ConfigurationDigest: strings.Repeat("b", 64), MemoryModelDigest: strings.Repeat("c", 64),
		SolverVersion: "solver", SolverDigest: strings.Repeat("d", 64), SemanticVersion: "semantic", SemanticDigest: strings.Repeat("e", 64), FootprintVersion: "footprint", FootprintDigest: strings.Repeat("f", 64),
	}
}
