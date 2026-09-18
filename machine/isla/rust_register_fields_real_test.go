//go:build isla_integration

// Fault-cause assertions use the structured register from the pinned model.
// The SDK must not replace model fields with register-specific constants.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustStructuredTrapCause(t *testing.T) {
	useTrapNotificationConfiguration(t)
	content := compileRustExecutable(t, "function_table.rs")
	boundary := rustFaultSafetyBoundary(t, content, "0x0")
	boundary.ForbiddenModelCalls = nil
	boundary.NegatedAssertion = "~(0:mcause.bits = 7)"
	proof := verifyRustPatternBoundary(t, content, boundary)
	if proof.Verification.Status != isla.Proved {
		t.Fatalf("declared store-access fault cause: %s", proof.Verification.Status)
	}
	boundary.NegatedAssertion = "~(0:mcause.bits = 8)"
	wrong := verifyRustPatternBoundary(t, content, boundary)
	if wrong.Verification.Status != isla.Disproved {
		t.Fatalf("changed cause requirement: %s", wrong.Verification.Status)
	}
	if !strings.Contains(wrong.Verification.CounterexampleState, "0:mcause.bits=#x0000000000000007;") {
		t.Fatalf("missing concrete cause: %s", wrong.Verification.CounterexampleState)
	}
	t.Logf("cause proof=%s counterexample=%s", proof.Verification.Status, wrong.Verification.CounterexampleState)
}

func TestRealRustStructuredCauseWithForbiddenTrap(t *testing.T) {
	useTrapNotificationConfiguration(t)
	content := compileRustExecutable(t, "function_table.rs")
	boundary := rustFaultSafetyBoundary(t, content, "0x0")
	boundary.NegatedAssertion = "~(0:mcause.bits = 7)"
	result := verifyRustPatternBoundary(t, content, boundary)
	if result.Verification.Status != isla.Disproved || !result.Verification.ModelCalls["trap_handler"] {
		t.Fatalf("correct cause erased forbidden trap: %#v", result.Verification)
	}
	if !strings.Contains(result.Verification.CounterexampleState, "0:mcause.bits=#x0000000000000007;") {
		t.Fatalf("trap has no concrete cause: %s", result.Verification.CounterexampleState)
	}
	t.Logf("forbidden trap: %s", result.Verification.CounterexampleState)
}
