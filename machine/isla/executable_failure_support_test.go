// Failure support creates generated programs and requests for wrapper checks.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func executableProgramRequest(t *testing.T, architecture isla.Artifact, configuration isla.Artifact) (isla.Program, isla.VerificationRequest) {
	t.Helper()
	content := machineFixture(t)
	program, err := isla.BuildProgram(content, uint64(len(content)), executableBoundary(0x80100000, "True"))
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "generated.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return program, executableRequest(t, path, program, architecture, configuration)
}

func executableVerifierWithTools(t *testing.T, architecture isla.Artifact, configuration isla.Artifact, solverName string, semanticName string) isla.ExecutableVerifier {
	t.Helper()
	verifier := executableBaseVerifierWithTools(t, solverName, semanticName)
	footprints := footprintEngine(t)
	release := footprintRelease(t, footprints, architecture, configuration)
	engine, err := isla.NewExecutableVerifier(verifier, footprints, release)
	if err != nil {
		t.Fatalf("NewExecutableVerifier() error = %v", err)
	}
	return engine
}
