//go:build isla_integration

// Compiler-built programs enter the same public API with explicit boundaries.
// A failed operation must not return a partial proof result.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func verifyRustBoundary(t *testing.T, content []byte, boundary isla.ProgramBoundary) isla.ExecutableResult {
	t.Helper()
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "program.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request, err := isla.NewVerificationRequest(realRequestPath(t, path), 2, 2048)
	if err != nil {
		t.Fatal(err)
	}
	verifier := realExecutableVerifier(t)
	limits := isla.ExecutableLimits{ThreadLimit: 2, TimeLimitSeconds: 60, MaximumOutputBytes: realELFOutputLimitBytes}
	result, err := verifier.VerifyProgram(t.Context(), request, program, limits)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
