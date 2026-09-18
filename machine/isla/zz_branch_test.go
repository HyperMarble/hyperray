//go:build isla_integration && arm64_acceptance

package isla_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

// The compiled branch function returns 17 when the input is zero, and
// 64 divided by the input otherwise.
func TestBranchFunction(t *testing.T) {
	content, err := os.ReadFile("/Volumes/Hak_SSD/hyperray-build/branch-test/branch-bin")
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	for _, testCase := range []struct {
		name   string
		input  uint64
		result uint64
	}{
		{"zero", 0, 17},
		{"four", 4, 16},
		{"seven", 7, 9},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assertion := fmt.Sprintf("~((0:R0 = 0x%016x & 0:_PC = 0x0000000100010000))", testCase.result)
			program := branchProgram(t, content, testCase.input, assertion)
			result := verifyLeakyReluProgram(t, program)
			if result.Verification.Status != isla.Proved {
				t.Fatalf("status = %s, want %s", result.Verification.Status, isla.Proved)
			}
		})
	}
}

func branchProgram(t *testing.T, content []byte, input uint64, assertion string) isla.Program {
	t.Helper()
	memory := branchMemory(t)
	boundary := isla.ARM64ProgramBoundary{
		Name: "arm64-branch", FunctionStart: 0x1000002e8, FunctionEnd: 0x100000300,
		ReturnAddress: 0x100010000, NegatedAssertion: assertion, MaximumProgramBytes: 1 << 20,
		Memory: &memory,
	}
	boundary.PostResetRegisters = []isla.RegisterValue{
		{Name: "R0", Value: fmt.Sprintf("0x%016x", input)},
		{Name: "R30", Value: "0x0000000100010000"},
	}
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	return program
}

// branchMemory declares the image and a stack, which the normal-memory
// profile requires even for a function that touches no memory.
func branchMemory(t *testing.T) isla.ARM64MemoryInput {
	t.Helper()
	mappings := []isla.MemoryMapping{
		{VA: 0x100000000, PA: 0x100000000, Length: 0x4000, Permission: isla.MemoryReadExecute},
		{VA: 0x100004000, PA: 0x100004000, Length: 0x4000, Permission: isla.MemoryRead},
		{VA: 0x3000, PA: 0x3000, Length: 0x1000, Permission: isla.MemoryReadWrite},
	}
	backing := []isla.MemoryBacking{{Address: 0x3bf0, Permission: isla.MemoryReadWrite, Bytes: make([]byte, 80)}}
	memory, err := isla.NewARM64MemoryInput(isla.ARM64NormalS1Fixed4KSIMD,
		isla.TableArena{Base: 0x5000, CapacityPages: 41}, mappings, backing)
	if err != nil {
		t.Fatalf("NewARM64MemoryInput() error = %v", err)
	}
	return memory
}
