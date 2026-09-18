//go:build isla_integration && arm64_acceptance

// This helper runs one genuine Leaky ReLU program with the normal ARM64 profile.
// It must return native errors instead of converting them to test success.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

// The unchanged prologue's measured footprint exceeds 68 MiB.
const leakyMaximumOutputBytes = 128 << 20

func verifyLeakyReluProgram(t *testing.T, program isla.Program) isla.ExecutableResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), "leaky-relu.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatalf("write Leaky ReLU program: %v", err)
	}
	request := arm64VerificationRequestWithOutputLimit(t, path, program, leakyMaximumOutputBytes)
	result, err := arm64NormalExecutableVerifier(t).VerifyProgram(t.Context(), request, program, isla.ExecutableLimits{
		ThreadLimit: 1, TimeLimitSeconds: 120, MaximumOutputBytes: leakyMaximumOutputBytes,
	})
	if err != nil {
		t.Fatalf("VerifyProgram() error = %v", err)
	}
	return result
}
