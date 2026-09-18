//go:build isla_integration

// Real executable helpers bind every Isla tool to one measured release.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func realExecutableVerifier(t *testing.T) isla.ExecutableVerifier {
	t.Helper()
	solver, err := isla.NewEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA"))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	semantics, err := isla.NewSemanticEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA_DUMP"))
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	verifier, err := isla.NewVerifier(solver, semantics)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	footprints, err := isla.NewFootprintEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA_FOOTPRINT"))
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	release := footprintRelease(t, footprints, realArtifact(t, "HYPERRAY_SAIL_IR"), realFootprintConfiguration(t))
	engine, err := isla.NewExecutableVerifier(verifier, footprints, release)
	if err != nil {
		t.Fatalf("NewExecutableVerifier() error = %v", err)
	}
	return engine
}
