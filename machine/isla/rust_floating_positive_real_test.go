//go:build isla_integration

// Positive floating tests require real compiler-built ELF state transitions.
// They do not replace the preserved reachable-unsupported-operation test.
package isla_test

import (
	"math"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustFloatingPointExactAddState(t *testing.T) {
	content := compileRustExecutable(t, "floating.rs")
	boundary := floatingBoundary(t, content, math.Float64bits(1), math.Float64bits(2), 0, math.Float64bits(3), 0)
	proof := verifyRustBoundary(t, content, boundary)
	if proof.Verification.Status != isla.Proved {
		t.Fatalf("exact add state: %#v", proof.Verification)
	}

	boundary.NegatedAssertion = floatingAssertion(math.Float64bits(4), 0, floatingDirtyMstatus())
	counterexample := verifyRustBoundary(t, content, boundary)
	if counterexample.Verification.Status != isla.Disproved {
		t.Fatalf("wrong exact result status: %#v", counterexample.Verification)
	}
	assertFloatingCounterexampleResult(t, counterexample, math.Float64bits(3))

	boundary.NegatedAssertion = floatingAssertion(math.Float64bits(3), 0, floatingCleanMstatus())
	wrongState := verifyRustBoundary(t, content, boundary)
	if wrongState.Verification.Status != isla.Disproved {
		t.Fatalf("wrong FS/SD state status: %#v", wrongState.Verification)
	}
	assertFloatingCounterexampleState(t, wrongState, "0:mstatus.bits=#x8000000000006000;")
}

func TestRealRustFloatingPointInexactAndStickyFlags(t *testing.T) {
	content := compileRustExecutable(t, "floating.rs")
	t.Run("inexact", func(t *testing.T) {
		boundary := floatingBoundary(t, content, math.Float64bits(1),
			math.Float64bits(math.Ldexp(1, -53)), 0, math.Float64bits(1), 1)
		proof := verifyRustBoundary(t, content, boundary)
		if proof.Verification.Status != isla.Proved {
			t.Fatalf("inexact add state: %#v", proof.Verification)
		}
	})
	t.Run("sticky", func(t *testing.T) {
		boundary := floatingBoundary(t, content, math.Float64bits(1), math.Float64bits(2), 0x10, math.Float64bits(3), 0x10)
		proof := verifyRustBoundary(t, content, boundary)
		if proof.Verification.Status != isla.Proved {
			t.Fatalf("sticky flags state: %#v", proof.Verification)
		}
	})
}
