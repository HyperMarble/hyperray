//go:build isla_integration

// Compiler-built programs expose feasible model calls through the public SDK.
// Fault safety must not depend on a handwritten instruction interpreter.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustForbiddenTrapCall(t *testing.T) {
	useTrapNotificationConfiguration(t)
	content := compileRustExecutable(t, "function_table.rs")
	boundary := rustFaultSafetyBoundary(t, content, "0x0")
	boundary.NegatedAssertion = "false"
	result := verifyRustPatternBoundary(t, content, boundary)
	if result.Verification.Status != isla.Disproved {
		t.Fatalf("reachable trap did not violate safety: %s", result.Verification.Status)
	}
	if !strings.Contains(result.Verification.CounterexampleState, "called:trap_handler=true;") {
		t.Fatalf("missing solver-selected trap: %s", result.Verification.CounterexampleState)
	}
	if !result.Verification.ModelCalls["trap_handler"] {
		t.Fatal("public result omitted the trap call")
	}
	t.Logf("fault witness: %s", result.Verification.CounterexampleState)
}

func TestRealRustNormalCallSafety(t *testing.T) {
	useTrapNotificationConfiguration(t)
	content := compileRustExecutable(t, "function_table.rs")
	boundary := rustFaultSafetyBoundary(t, content, "0x80410000")
	boundary.NegatedAssertion = "~(0:x10 = 17)"
	result := verifyRustPatternBoundary(t, content, boundary)
	if result.Verification.Status != isla.Proved {
		t.Fatalf("normal return violated safety: %#v", result.Verification)
	}
	boundary.NegatedAssertion = "0:x10 = 17"
	wrong := verifyRustPatternBoundary(t, content, boundary)
	if wrong.Verification.Status != isla.Disproved {
		t.Fatalf("wrong return requirement did not fail: %s", wrong.Verification.Status)
	}
	if !strings.Contains(wrong.Verification.CounterexampleState, "called:trap_handler=false;") {
		t.Fatalf("normal return has no call observation: %s", wrong.Verification.CounterexampleState)
	}
	called, present := wrong.Verification.ModelCalls["trap_handler"]
	if !present || called {
		t.Fatal("normal return has the wrong public call value")
	}
	t.Logf("normal witness: %s", wrong.Verification.CounterexampleState)
}
