//go:build isla_integration

// The real generated-program test starts from ELF bytes and ends at a verdict.
// No instruction mnemonic or source-function rule enters the production path.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealGeneratedELFProgram(t *testing.T) {
	program, proof := verifyGeneratedELF(t, "~(0:x11 = 7)")
	if proof.Verification.Status != isla.Proved || len(proof.Footprints.Instructions) != int(program.InstructionCount()) {
		t.Errorf("generated ELF proof = %#v", proof)
	}
	_, counterexample := verifyGeneratedELF(t, "~(0:x11 = 8)")
	if counterexample.Verification.Status != isla.Disproved || counterexample.Verification.CounterexampleState == "" {
		t.Errorf("generated ELF counterexample = %#v", counterexample)
	}
}

func verifyGeneratedELF(t *testing.T, assertion string) (isla.Program, isla.ExecutableResult) {
	t.Helper()
	content := machineFixture(t)
	program, err := isla.BuildProgram(content, uint64(len(content)), executableBoundary(0x80100000, assertion))
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "generated.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	request, err := isla.NewVerificationRequest(realRequestPath(t, path), 2, 2048)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	verifier := realExecutableVerifier(t)
	limits := isla.ExecutableLimits{ThreadLimit: 2, TimeLimitSeconds: 60, MaximumOutputBytes: realELFOutputLimitBytes}
	result, err := verifier.VerifyProgram(t.Context(), request, program, limits)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	return program, result
}
