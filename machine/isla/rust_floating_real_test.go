//go:build isla_integration

// This test proves that model-load diagnostics do not block supported add.
// It does not change or cover the separate trap_callback execution path.
package isla_test

import (
	"math"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustFloatingPointLoadWarningsDoNotBlockSupportedAdd(t *testing.T) {
	content := compileRustExecutable(t, "floating.rs")
	boundary := floatingBoundary(t, content, math.Float64bits(1), math.Float64bits(2), 0, math.Float64bits(3), 0)
	proof := verifyRustBoundary(t, content, boundary)
	if proof.Verification.Status != isla.Proved {
		t.Fatalf("supported add relation: %#v", proof.Verification)
	}
	coverage := proof.StaticCoverage
	if !coverage.Complete || coverage.CoveredInstructions == 0 || coverage.TotalInstructions == 0 {
		t.Fatalf("supported add coverage = %#v", coverage)
	}
	for _, instruction := range proof.Footprints.Instructions {
		if strings.Contains(instruction.Diagnostics, "softfloat_f64add") {
			t.Fatalf("supported add has an unavailable-primitive warning: %s", instruction.Diagnostics)
		}
	}

	boundary.NegatedAssertion = floatingAssertion(math.Float64bits(4), 0, floatingDirtyMstatus())
	counterexample := verifyRustBoundary(t, content, boundary)
	if counterexample.Verification.Status != isla.Disproved {
		t.Fatalf("wrong add result status: %#v", counterexample.Verification)
	}
	assertFloatingCounterexampleResult(t, counterexample, math.Float64bits(3))
}
