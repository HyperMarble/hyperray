//go:build isla_integration && arm64_acceptance

// This external test runs the genuine Leaky ReLU Mach-O through ARM64 Isla.
// It must report a real solver result and must not create a buggy binary.
package isla_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

const leakyReturnAddress = uint64(0x100010000)

func TestRealARM64LeakyRelu(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "machine", "arm64", "leaky_relu", "leaky-relu-arm64-static"))
	if err != nil {
		t.Fatalf("read Leaky ReLU Mach-O: %v", err)
	}
	for _, testCase := range leakyCases(t) {
		t.Run(testCase.Name, func(t *testing.T) {
			memory := leakyMemoryInput(t, testCase.InputWords)
			correct := leakyProgram(t, content, memory, leakyViolation(testCase.ExpectedWords, testCase.NegativeCount))
			correctResult := verifyLeakyReluProgram(t, correct)
			if correctResult.Verification.Status != isla.Proved {
				t.Fatalf("correct Leaky ReLU property status = %s, want %s", correctResult.Verification.Status, isla.Proved)
			}
			wrongExpected := append([]string(nil), testCase.ExpectedWords...)
			wrongExpected[0] = "0xdeadbeef"
			wrong := leakyProgram(t, content, memory, leakyViolation(wrongExpected, testCase.NegativeCount))
			wrongResult := verifyLeakyReluProgram(t, wrong)
			if wrongResult.Verification.Status != isla.Disproved {
				t.Fatalf("negative Leaky ReLU property status = %s, want %s", wrongResult.Verification.Status, isla.Disproved)
			}
		})
	}
}

func leakyProgram(t *testing.T, content []byte, memory isla.ARM64MemoryInput, assertion string) isla.Program {
	t.Helper()
	boundary := isla.ARM64ProgramBoundary{Name: "arm64-genuine-leaky-relu", FunctionStart: 0x1000002f8, FunctionEnd: 0x100000398, ReturnAddress: leakyReturnAddress, NegatedAssertion: assertion, MaximumProgramBytes: 1 << 20, Memory: &memory, MemoryObservations: leakyObservations()}
	boundary.PostResetRegisters = []isla.RegisterValue{{Name: "R0", Value: "0x400000"}, {Name: "R30", Value: fmt.Sprintf("0x%x", leakyReturnAddress)}, {Name: "SP_EL0", Value: "0x3c40"}}
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	return program
}

func leakyMappings() []isla.MemoryMapping {
	return []isla.MemoryMapping{
		{VA: 0x100000000, PA: 0x100000000, Length: 0x4000, Permission: isla.MemoryReadExecute},
		{VA: 0x100004000, PA: 0x100004000, Length: 0x4000, Permission: isla.MemoryRead},
		{VA: 0x400000, PA: 0x400000, Length: 0x1000, Permission: isla.MemoryReadWrite},
		{VA: 0x3000, PA: 0x3000, Length: 0x1000, Permission: isla.MemoryReadWrite},
	}
}

func leakyViolation(expected []string, negativeCount int) string {
	parts := []string{fmt.Sprintf("0:R0 = %d", negativeCount), "0:SP_EL0 = 0x0000000000003c40", "0:_PC = 0x0000000100010000"}
	for index, word := range expected {
		parts = append(parts, fmt.Sprintf("*lane%d = %s", index, word))
	}
	return "~((" + strings.Join(parts, " & ") + "))"
}
