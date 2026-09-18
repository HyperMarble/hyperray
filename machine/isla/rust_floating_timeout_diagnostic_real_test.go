//go:build isla_integration

package isla_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustFloatingPointSymbolicTimeoutDiagnostic(t *testing.T) {
	content := compileRustExecutable(t, "floating.rs")
	boundary := floatingBoundary(t, content, math.Float64bits(1), math.Float64bits(2), 0, math.Float64bits(3), 0)
	boundary.InitialRegisters = boundary.InitialRegisters[:2]
	boundary.NegatedAssertion = "0:f11 = 0x4000000000000000"
	input, err := verifyRustBoundaryWithSemanticTimeout(t, content, boundary, 60)
	if err != nil {
		t.Fatalf("symbolic antecedent diagnostic: semantic timeout=60s error=%v", err)
	}
	if input.Verification.Status != isla.Disproved {
		t.Fatalf("symbolic antecedent diagnostic status: %#v", input.Verification)
	}
	if !strings.Contains(input.Verification.CounterexampleState, "0:f11=#x4000000000000000;") {
		t.Fatalf("symbolic antecedent diagnostic state: %s", input.Verification.CounterexampleState)
	}
	boundary.NegatedAssertion = "0:f11 = 0x4000000000000000 & ~(0:f10 = 0x4008000000000000 & 0:fcsr.bits = 0x00000000 & 0:mstatus.bits = 0x8000000000006000)"
	result, err := verifyRustBoundaryWithSemanticTimeout(t, content, boundary, 60)
	if err != nil {
		t.Fatalf("symbolic relation diagnostic: semantic timeout=60s error=%v", err)
	}
	if result.Verification.Status != isla.Proved {
		t.Fatalf("symbolic relation diagnostic status: %#v", result.Verification)
	}
	t.Logf("symbolic relation diagnostic: semantic timeout=60s status=%s", result.Verification.Status)
}

func verifyRustBoundaryWithSemanticTimeout(t *testing.T, content []byte, boundary isla.ProgramBoundary, seconds uint64) (isla.ExecutableResult, error) {
	t.Helper()
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		return isla.ExecutableResult{}, err
	}
	path := filepath.Join(t.TempDir(), "program.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		return isla.ExecutableResult{}, err
	}
	request := realRequestPathWithTimeout(t, path, 2, seconds)
	verification, err := isla.NewVerificationRequest(request, 2, 2048)
	if err != nil {
		return isla.ExecutableResult{}, err
	}
	return realExecutableVerifier(t).VerifyProgram(t.Context(), verification, program, isla.ExecutableLimits{
		ThreadLimit: 2, TimeLimitSeconds: 60, MaximumOutputBytes: realELFOutputLimitBytes,
	})
}

func realRequestPathWithTimeout(t *testing.T, programPath string, pcVisitLimit, seconds uint64) isla.Request {
	t.Helper()
	request, err := isla.NewRequest(realArtifact(t, "HYPERRAY_SAIL_IR"), realArtifact(t, "HYPERRAY_ISLA_CONFIG"), realArtifact(t, "HYPERRAY_MEMORY_MODEL"), identifiedArtifact(t, programPath), pcVisitLimit, seconds, 16<<20)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
