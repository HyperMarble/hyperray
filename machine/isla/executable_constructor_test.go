// Constructor tests reject incomplete, mixed-tool, and mixed-version engines.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestExecutableVerifierRejectsIncompleteParts(t *testing.T) {
	if _, err := isla.NewExecutableVerifier(isla.Verifier{}, isla.FootprintEngine{}, isla.FootprintRelease{}); err == nil {
		t.Error("NewExecutableVerifier() accepted an empty verifier")
	}
	if _, err := isla.NewExecutableVerifier(testVerifier(t), footprintEngine(t), isla.FootprintRelease{}); err == nil {
		t.Error("NewExecutableVerifier() accepted an empty release")
	}
}

func TestExecutableVerifierRejectsDifferentFootprintTool(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	first := footprintEngine(t)
	release := footprintRelease(t, first, architecture, configuration)
	second := copiedFootprintEngine(t)
	if _, err := isla.NewExecutableVerifier(executableBaseVerifier(t), second, release); err == nil {
		t.Error("NewExecutableVerifier() accepted a different footprint tool")
	}
}

func TestExecutableVerifierRejectsDifferentVersions(t *testing.T) {
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	engine := footprintEngine(t)
	release := footprintRelease(t, engine, architecture, configuration)
	if _, err := isla.NewExecutableVerifier(testVerifier(t), engine, release); err == nil {
		t.Error("NewExecutableVerifier() accepted different tool versions")
	}
}

func copiedFootprintEngine(t *testing.T) isla.FootprintEngine {
	t.Helper()
	content, err := os.ReadFile(footprintTool(t))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "isla-footprint")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	engine, err := isla.NewFootprintEngine(t.Context(), path)
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	return engine
}
