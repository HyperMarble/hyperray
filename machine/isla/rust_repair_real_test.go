//go:build isla_integration

// The public SDK evaluates a buggy stack update and its repair under one requirement.
// A repair must change the compiled program, not weaken the requirement.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustStackRepairKeepsRequirement(t *testing.T) {
	brokenCode := compileRustExecutable(t, "stack_array_bug.rs")
	brokenBoundary := rustProgramBoundary(t, brokenCode, 2, 13)
	brokenBoundary.MemoryProfile = isla.SequentialMemory
	brokenBoundary.InitialRegisters = append(brokenBoundary.InitialRegisters,
		isla.RegisterValue{Name: "x2", Value: "0x80410000"})
	broken := verifyRustPatternBoundary(t, brokenCode, brokenBoundary)
	assertSymbolicCounterexample(t, broken, 12)

	repairedCode := compileRustExecutable(t, "stack_array.rs")
	repairedBoundary := rustProgramBoundary(t, repairedCode, 2, 13)
	repairedBoundary.MemoryProfile = brokenBoundary.MemoryProfile
	repairedBoundary.InitialRegisters = append(repairedBoundary.InitialRegisters,
		isla.RegisterValue{Name: "x2", Value: "0x80410000"})
	if repairedBoundary.NegatedAssertion != brokenBoundary.NegatedAssertion {
		t.Fatal("repair changed the requirement")
	}
	repaired := verifyRustPatternBoundary(t, repairedCode, repairedBoundary)
	if repaired.Verification.Status != isla.Proved {
		t.Fatalf("repair did not satisfy the requirement: %#v", repaired.Verification)
	}
	if repaired.Verification.SolverEvidence.ProgramDigest == broken.Verification.SolverEvidence.ProgramDigest {
		t.Fatal("repair retained the buggy program identity")
	}
	t.Logf("same requirement %s: buggy=%s repaired=%s",
		brokenBoundary.NegatedAssertion, broken.Verification.Status, repaired.Verification.Status)
}
