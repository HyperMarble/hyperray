//go:build isla_integration

package isla_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustFloatingPointSymbolicInputs(t *testing.T) {
	content := compileRustExecutable(t, "floating.rs")
	boundary := floatingBoundary(t, content, 0, 0, 0, math.Float64bits(3), 0)
	boundary.InitialRegisters = boundary.InitialRegisters[:1]
	boundary.NegatedAssertion = "0:f10 = 0x4008000000000000"
	result := verifyRustBoundary(t, content, boundary)
	if result.Verification.Status != isla.Disproved {
		t.Fatalf("symbolic result status: %#v", result.Verification)
	}
	if !strings.Contains(result.Verification.CounterexampleState, "0:f10=#x4008000000000000;") {
		t.Fatalf("symbolic result state: %s", result.Verification.CounterexampleState)
	}
}

func TestRealRustFloatingPointBoundedSymbolicRelation(t *testing.T) {
	content := compileRustExecutable(t, "floating.rs")
	boundary := floatingBoundary(t, content, math.Float64bits(1), math.Float64bits(2), 0, math.Float64bits(3), 0)
	boundary.InitialRegisters = boundary.InitialRegisters[:2]
	boundary.NegatedAssertion = "0:f11 = 0x4000000000000000"
	input := verifyRustBoundary(t, content, boundary)
	if input.Verification.Status != isla.Disproved {
		t.Fatalf("symbolic antecedent is not satisfiable: %#v", input.Verification)
	}
	assertFloatingCounterexampleState(t, input, "0:f11=#x4000000000000000;")

	boundary.NegatedAssertion = "0:f11 = 0x4000000000000000 & ~(0:f10 = 0x4008000000000000 & 0:fcsr.bits = 0x00000000 & 0:mstatus.bits = 0x8000000000006000)"
	proof := verifyRustBoundary(t, content, boundary)
	if proof.Verification.Status != isla.Proved {
		t.Fatalf("bounded symbolic relation: %#v", proof.Verification)
	}

	boundary.NegatedAssertion = "0:f11 = 0x4000000000000000 & ~(0:f10 = 0x4010000000000000 & 0:fcsr.bits = 0x00000000 & 0:mstatus.bits = 0x8000000000006000)"
	wrong := verifyRustBoundary(t, content, boundary)
	if wrong.Verification.Status != isla.Disproved {
		t.Fatalf("wrong bounded symbolic relation status: %#v", wrong.Verification)
	}
	assertFloatingCounterexampleResult(t, wrong, math.Float64bits(3))
}

func assertFloatingCounterexampleResult(t *testing.T, result isla.ExecutableResult, expected uint64) {
	t.Helper()
	wanted := fmt.Sprintf("0:f10=#x%016x;", expected)
	assertFloatingCounterexampleState(t, result, wanted)
}

func assertFloatingCounterexampleState(t *testing.T, result isla.ExecutableResult, wanted string) {
	t.Helper()
	if !strings.Contains(result.Verification.CounterexampleState, wanted) {
		t.Fatalf("counterexample state: %s", result.Verification.CounterexampleState)
	}
}
