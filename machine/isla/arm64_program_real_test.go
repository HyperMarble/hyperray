//go:build isla_integration && arm64_acceptance

// This test sends the checked-in ARM64 Mach-O through Isla and SMT.
// It must not rebuild, execute, or replace the original image.
package isla_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

const arm64FixtureDigest = "a55e2554b91b1a5d451df1b1125a883f9f239f610ba841caed55a75651574ac4"

// Mach-O materializes 32768 bytes: 16384 __TEXT plus 16384 __LINKEDIT.
const arm64LoadedBytes = uint64(32768)

func TestRealARM64MachOProgram(t *testing.T) {
	content, functionStart, functionEnd := arm64Fixture(t)
	returnAddress := uint64(0x100008000)
	assertARM64ContinuationOutsideImage(t, returnAddress)
	contentDigest := sha256.Sum256(content)
	if hex.EncodeToString(contentDigest[:]) != arm64FixtureDigest {
		t.Fatalf("fixture SHA-256 = %s, want %s", hex.EncodeToString(contentDigest[:]), arm64FixtureDigest)
	}
	boundary := arm64Boundary(functionStart, functionEnd, returnAddress, 4)
	program, err := isla.BuildARM64Program(content, arm64LoadedBytes, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	if program.ImageDigest() != arm64FixtureDigest {
		t.Fatalf("Program.ImageDigest() = %s, want %s", program.ImageDigest(), arm64FixtureDigest)
	}
	proof := verifyARM64Program(t, program)
	assertARM64StatusCounts(t, proof, isla.Proved)
	assertARM64TerminalEvidence(t, proof, returnAddress)
	if proof.Program.ImageDigest != arm64FixtureDigest {
		t.Fatalf("proof image digest = %s, want %s", proof.Program.ImageDigest, arm64FixtureDigest)
	}

	wrongBoundary := arm64Boundary(functionStart, functionEnd, returnAddress, 5)
	wrongProgram, err := isla.BuildARM64Program(content, arm64LoadedBytes, wrongBoundary)
	if err != nil {
		t.Fatalf("BuildARM64Program(wrong result) error = %v", err)
	}
	counterexample := verifyARM64Program(t, wrongProgram)
	assertARM64StatusCounts(t, counterexample, isla.Disproved)
	if counterexample.Program.ImageDigest != arm64FixtureDigest {
		t.Fatalf("counterexample image digest = %s, want %s", counterexample.Program.ImageDigest, arm64FixtureDigest)
	}
	assertARM64Witness(t, counterexample.Verification.CounterexampleState, returnAddress)
	assertARM64TerminalEvidence(t, counterexample, returnAddress)
}

func arm64Boundary(start uint64, end uint64, returnAddress uint64, expected uint64) isla.ARM64ProgramBoundary {
	return isla.ARM64ProgramBoundary{
		Name: "arm64-closed-leaf", FunctionStart: start, FunctionEnd: end,
		ReturnAddress: returnAddress, PostResetRegisters: []isla.RegisterValue{
			{Name: "R0", Value: "3"}, {Name: "R30", Value: fmt.Sprintf("0x%x", returnAddress)},
			{Name: "SP_EL0", Value: "0x3c40"},
		},
		NegatedAssertion: arm64Violation(expected, returnAddress), MaximumProgramBytes: 1 << 20,
	}
}

func arm64Violation(expected uint64, returnAddress uint64) string {
	return fmt.Sprintf("~((0:X0 = %d) & (0:SP_EL0 = 0x0000000000003c40) & (0:_PC = 0x%016x))", expected, returnAddress)
}
