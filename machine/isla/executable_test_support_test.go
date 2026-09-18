// Executable test support constructs all engines through their public APIs.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func executableVerifier(t *testing.T, architecture isla.Artifact, configuration isla.Artifact) isla.ExecutableVerifier {
	t.Helper()
	verifier := executableBaseVerifier(t)
	footprints := footprintEngine(t)
	release := footprintRelease(t, footprints, architecture, configuration)
	executable, err := isla.NewExecutableVerifier(verifier, footprints, release)
	if err != nil {
		t.Fatalf("NewExecutableVerifier() error = %v", err)
	}
	return executable
}

func executableBaseVerifier(t *testing.T) isla.Verifier {
	t.Helper()
	return executableBaseVerifierWithTools(t, "fake-isla-executable.sh", "fake-isla-dump-executable.sh")
}

func executableBaseVerifierWithTools(t *testing.T, solverName string, semanticName string) isla.Verifier {
	t.Helper()
	solver, err := isla.NewEngine(t.Context(), semanticTool(t, solverName))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	semantics, err := isla.NewSemanticEngine(t.Context(), semanticTool(t, semanticName))
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	verifier, err := isla.NewVerifier(solver, semantics)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	return verifier
}
