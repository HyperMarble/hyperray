//go:build isla_integration

// Exhausted instruction visits must stop the unchanged recursive program.
// A smaller search limit must never produce a proof result.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustRecursionRejectsVisitLimit(t *testing.T) {
	content := compileRustExecutable(t, "recursive_calls.rs")
	boundary := rustProgramBoundary(t, content, 3, 6)
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.InitialRegisters = append(boundary.InitialRegisters,
		isla.RegisterValue{Name: "x2", Value: "0x80410000"})
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "limited-recursion.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	const visitsBeforeRecursiveBaseCase = 2
	request := rustPatternVisitRequest(t, path, visitsBeforeRecursiveBaseCase)
	verifier := realExecutableVerifier(t)
	limits := isla.ExecutableLimits{
		ThreadLimit: rustPatternThreadLimit, TimeLimitSeconds: rustPatternTimeLimitSeconds,
		MaximumOutputBytes: rustPatternOutputLimitBytes,
	}
	result, err := verifier.VerifyProgram(t.Context(), request, program, limits)
	if err == nil || result.Verification.Status != "" {
		t.Fatalf("exhausted visit limit returned a result: result=%#v error=%v", result, err)
	}
	if !strings.Contains(err.Error(), "PCLimitReached(") {
		t.Fatalf("missing visit-limit error: %v", err)
	}
	if !strings.Contains(err.Error(), "Error during trace setup:") {
		t.Fatalf("visit limit did not stop the semantic stage: %v", err)
	}
}
