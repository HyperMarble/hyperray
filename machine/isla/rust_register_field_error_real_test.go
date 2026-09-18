//go:build isla_integration

// A field absent from its declared structure must fail before execution.
// The caller must not receive a partial proof result.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustRejectsMissingRegisterField(t *testing.T) {
	content := compileRustExecutable(t, "branch.rs")
	boundary := rustProgramBoundary(t, content, 0, 17)
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.NegatedAssertion = "~(0:mcause.mepc = 7)"
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "missing-field.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request := rustPatternRequest(t, path)
	verifier := realExecutableVerifier(t)
	limits := isla.ExecutableLimits{ThreadLimit: rustPatternThreadLimit,
		TimeLimitSeconds: rustPatternTimeLimitSeconds, MaximumOutputBytes: rustPatternOutputLimitBytes}
	result, err := verifier.VerifyProgram(t.Context(), request, program, limits)
	if err == nil || result.Verification.Status != "" {
		t.Fatalf("missing field produced a result: %#v, %v", result, err)
	}
	if !strings.Contains(err.Error(), "Register field is absent from its declared struct") {
		t.Fatalf("missing field has no declared-type error: %v", err)
	}
	t.Logf("field rejection: %v", err)
}
