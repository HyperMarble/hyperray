//go:build isla_integration

// A symbolic register lets the model explore both compiled branch paths.
// These assertions concern the model domain, not compiler correctness.
package isla_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustSymbolicBranchInput(t *testing.T) {
	content := compileRustExecutable(t, "branch.rs")
	boundary := rustProgramBoundary(t, content, 0, 0)
	boundary.InitialRegisters = boundary.InitialRegisters[:1]
	boundary.NegatedAssertion = "0:x10 = 65"
	result := verifyRustBoundary(t, content, boundary)
	if result.Verification.Status != isla.Proved {
		t.Errorf("impossible result status = %s", result.Verification.Status)
	}
	assertRustExecutionInventory(t, result, false)
	if !strings.Contains(result.Verification.Semantics.TraceOutput, "(read-reg |x10| nil v") {
		t.Error("semantic output has no symbolic input-register read")
	}
	for _, value := range []uint64{17, 64} {
		boundary.NegatedAssertion = fmt.Sprintf("0:x10 = %d", value)
		counterexample := verifyRustBoundary(t, content, boundary)
		assertSymbolicCounterexample(t, counterexample, value)
	}
}

func assertSymbolicCounterexample(t *testing.T, result isla.ExecutableResult, value uint64) {
	t.Helper()
	if result.Verification.Status != isla.Disproved {
		t.Errorf("branch outcome has no counterexample: status %s", result.Verification.Status)
	}
	wanted := fmt.Sprintf("0:x10=#x%016x;", value)
	if result.Verification.CounterexampleState != wanted {
		t.Errorf("counterexample = %q, want %q", result.Verification.CounterexampleState, wanted)
	}
	t.Logf("counterexample: %s", result.Verification.CounterexampleState)
}
