//go:build isla_integration

// Stress fixtures enter the public executable API with declared resource limits.
// This helper must not weaken semantic or instruction-coverage conditions.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func verifyRustPatternBoundary(t *testing.T, content []byte, boundary isla.ProgramBoundary) isla.ExecutableResult {
	t.Helper()
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "program.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request := rustPatternRequest(t, path)
	verifier := realExecutableVerifier(t)
	limits := isla.ExecutableLimits{
		ThreadLimit: rustPatternThreadLimit, TimeLimitSeconds: rustPatternTimeLimitSeconds,
		MaximumOutputBytes: rustPatternOutputLimitBytes,
	}
	result, err := verifier.VerifyProgram(t.Context(), request, program, limits)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verification.Semantics.Evidence.PCVisitLimit != rustPatternPCVisitLimit ||
		result.Verification.SolverEvidence.PCVisitLimit != rustPatternPCVisitLimit {
		t.Fatalf("execution stages returned different visit guards: %#v", result.Verification)
	}
	return result
}
