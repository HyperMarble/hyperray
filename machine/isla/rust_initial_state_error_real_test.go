//go:build isla_integration

// A wrong structured state must fail before it can constrain a program.
// The caller must receive an error with no partial verification result.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustRejectsWrongTypedState(t *testing.T) {
	useTrapNotificationConfiguration(t)
	content := compileRustExecutable(t, "function_table.rs")
	boundary := rustProgramBoundary(t, content, 0, 0)
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.InitialState = []isla.RegisterValue{{Name: "mtvec", Value: "{ bits = true }"}}
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wrong-state.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request := rustPatternRequest(t, path)
	verifier := realExecutableVerifier(t)
	limits := isla.ExecutableLimits{ThreadLimit: rustPatternThreadLimit,
		TimeLimitSeconds: rustPatternTimeLimitSeconds, MaximumOutputBytes: rustPatternOutputLimitBytes}
	result, err := verifier.VerifyProgram(t.Context(), request, program, limits)
	if err == nil || result.Verification.Status != "" {
		t.Fatalf("wrong state produced a verdict: %#v, %v", result, err)
	}
	if !strings.Contains(err.Error(), "Initial state for mtvec does not match its declared type") {
		t.Fatalf("wrong state lacks its type error: %v", err)
	}
	t.Logf("typed state rejection: %v", err)
}
