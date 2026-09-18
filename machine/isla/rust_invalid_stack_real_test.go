//go:build isla_integration

// An invalid stack must remain an error instead of an excluded execution.
// This uses the same compiler-built fixture and public proof API.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustSequentialInvalidStack(t *testing.T) {
	content := compileRustExecutable(t, "stack_values.rs")
	boundary := rustProgramBoundary(t, content, 2, 13)
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.InitialRegisters = append(boundary.InitialRegisters,
		isla.RegisterValue{Name: "x2", Value: "0x0"})
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "invalid-stack.toml")
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
	if err == nil || result.Verification.Status != "" {
		t.Fatalf("invalid stack was not an explicit engine error: result=%#v error=%v", result, err)
	}
	for _, required := range []string{"Error during trace setup:", "NoFunction(\"trap_callback\"", "zhandle_mem_exception"} {
		if !strings.Contains(err.Error(), required) {
			t.Fatalf("invalid stack lacks %q in its fault evidence: %v", required, err)
		}
	}
	t.Logf("invalid stack: %v", err)
}
