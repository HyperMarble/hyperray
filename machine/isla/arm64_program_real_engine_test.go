//go:build isla_integration && arm64_acceptance

// This helper constructs the real ARM64 Isla execution engines.
// It must not substitute a fake engine or a different machine profile.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func arm64ExecutableVerifier(t *testing.T) isla.ExecutableVerifier {
	return arm64ExecutableVerifierWithManifest(t, "HYPERRAY_ARM64_EXECUTION_MANIFEST", "HYPERRAY_ARM64_EXECUTION_MANIFEST_SHA256")
}

func arm64NormalExecutableVerifier(t *testing.T) isla.ExecutableVerifier {
	return arm64ExecutableVerifierWithManifest(t, "HYPERRAY_ARM64_NORMAL_EXECUTION_MANIFEST", "HYPERRAY_ARM64_NORMAL_EXECUTION_MANIFEST_SHA256")
}

func arm64ExecutableVerifierWithManifest(t *testing.T, manifestVariable string, digestVariable string) isla.ExecutableVerifier {
	t.Helper()
	solver, err := isla.NewEngine(t.Context(), requiredPath(t, "HYPERRAY_ARM64_ISLA"))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	semantics, err := isla.NewSemanticEngine(t.Context(), requiredPath(t, "HYPERRAY_ARM64_ISLA_DUMP"))
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	verifier, err := isla.NewVerifier(solver, semantics)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	footprints, err := isla.NewFootprintEngine(t.Context(), requiredPath(t, "HYPERRAY_ARM64_ISLA_FOOTPRINT"))
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	architecture := arm64Artifact(t, "HYPERRAY_ARM64_SAIL_IR")
	configuration := arm64Artifact(t, "HYPERRAY_ARM64_ISLA_CONFIG")
	release := footprintRelease(t, footprints, architecture, configuration)
	manifestDigest := requiredPath(t, digestVariable)
	manifest, err := isla.NewArtifact(requiredPath(t, manifestVariable), manifestDigest)
	if err != nil {
		t.Fatalf("NewArtifact(ARM64 execution manifest) error = %v", err)
	}
	capability, err := isla.NewARM64ExecutionCapability(manifest, verifier, footprints, architecture, configuration, arm64Artifact(t, "HYPERRAY_ARM64_MEMORY_MODEL"))
	if err != nil {
		t.Fatalf("NewARM64ExecutionCapability() error = %v", err)
	}
	engine, err := isla.NewExecutableVerifierWithCapability(verifier, footprints, release, capability)
	if err != nil {
		t.Fatalf("NewExecutableVerifier() error = %v", err)
	}
	return engine
}
