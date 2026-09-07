//go:build isla_integration

// This test distinguishes model-load diagnostics from reachable operations.
// A reachable unavailable primitive must return an explicit process error.
package isla_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustFloatingPointSeparatesLoadAndReachability(t *testing.T) {
	content := compileRustExecutable(t, "floating.rs")
	boundary := rustProgramBoundary(t, content, 0, 0)
	boundary.InitialRegisters[1] = isla.RegisterValue{Name: "f10", Value: "0"}
	boundary.InitialRegisters = append(boundary.InitialRegisters, isla.RegisterValue{Name: "f11", Value: "0"})
	boundary.InitialState = []isla.RegisterValue{
		{Name: "misa", Value: "{ bits = 0x000000000000112d }"},
		{Name: "mstatus", Value: "{ bits = 0x0000000000006000 }"},
	}
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "floating.toml")
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
	var failure *isla.Error
	if err == nil || result.Program.ProgramDigest != "" || !errors.As(err, &failure) {
		t.Fatalf("floating operation result = %#v, error = %v", result, err)
	}
	if failure.Code != isla.ProcessFail || !strings.Contains(failure.Detail, `NoFunction("extern_f64Add"`) {
		t.Fatalf("floating operation error = %v, want reachable extern_f64Add", failure)
	}
	if !strings.Contains(failure.Detail, "No primop softfloat_f64add") {
		t.Fatalf("floating operation error = %v, missing load-time softfloat diagnostic", failure)
	}
	t.Logf("load-time diagnostic and reachable softfloat wrapper: %v", failure)
}
