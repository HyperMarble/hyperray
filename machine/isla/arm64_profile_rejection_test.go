// Unsupported ARM64 profiles fail before any RISC-V verifier operation.
// This keeps the temporary native capability boundary explicit and observable.
package isla

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyProgramRejectsARM64BeforeFootprint(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "machine", "arm64", "tiny-arm64-static"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	program, err := BuildARM64Program(content, 32768, ARM64ProgramBoundary{
		Name: "unsupported-arm64", FunctionStart: 0x1000002e8, FunctionEnd: 0x1000002f0,
		ReturnAddress: 0x100008000, PostResetRegisters: []RegisterValue{
			{Name: "R0", Value: "3"}, {Name: "R30", Value: "0x100008000"},
		}, NegatedAssertion: "True", MaximumProgramBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	_, err = (ExecutableVerifier{}).VerifyProgram(t.Context(), VerificationRequest{}, program, ExecutableLimits{})
	var engineFailure *Error
	if !errors.As(err, &engineFailure) || engineFailure.Code != UnsupportedProfile {
		t.Fatalf("VerifyProgram() error = %v, want %s", err, UnsupportedProfile)
	}
}
