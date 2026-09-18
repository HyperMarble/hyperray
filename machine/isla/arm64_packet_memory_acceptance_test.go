//go:build isla_integration && arm64_acceptance

// This helper binds every packet case to the reviewed finite memory contract.
// It must keep all 32 bytes and the explicit 80-byte zero stack backing.
package isla_test

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

// The native builder's conservative budget is 4*N+1 for N declared 4K pages.
// The packet image declares ten pages: eight image, one packet, and one stack.
const packetTableCapacityPages = uint32(41)

func buildPacketMemoryProgram(t *testing.T, content []byte, start uint64, end uint64, testCase packetCase, expected uint64) isla.Program {
	t.Helper()
	bytesIn, err := hex.DecodeString(testCase.Bytes)
	if err != nil || len(bytesIn) != 32 {
		t.Fatalf("case %q packet bytes = %q, want 32 bytes: %v", testCase.Name, testCase.Bytes, err)
	}
	memory, err := packetMemoryInput(bytesIn)
	if err != nil {
		t.Fatalf("case %q memory input: %v", testCase.Name, err)
	}
	boundary := isla.ARM64ProgramBoundary{
		Name: "arm64-bounded-packet", FunctionStart: start, FunctionEnd: end,
		ReturnAddress: packetReturn, Memory: &memory,
		PostResetRegisters: []isla.RegisterValue{
			{Name: "R0", Value: "0x400000"}, {Name: "R1", Value: strconv.Itoa(testCase.InputLength)},
			{Name: "R30", Value: fmt.Sprintf("0x%x", packetReturn)}, {Name: "SP_EL0", Value: "0x3c40"},
		},
		NegatedAssertion: packetViolation(expected), MaximumProgramBytes: 1 << 20,
	}
	program, err := isla.BuildARM64Program(content, 1<<20, boundary)
	if err != nil {
		t.Fatalf("case %q BuildARM64Program() error = %v", testCase.Name, err)
	}
	return program
}

func packetMemoryInput(bytesIn []byte) (isla.ARM64MemoryInput, error) {
	return isla.NewARM64MemoryInput(
		isla.ARM64NormalS1Fixed4KSIMD, isla.TableArena{Base: 0x5000, CapacityPages: packetTableCapacityPages},
		[]isla.MemoryMapping{
			{VA: 0x100000000, PA: 0x100000000, Length: 0x4000, Permission: isla.MemoryReadExecute},
			{VA: 0x100004000, PA: 0x100004000, Length: 0x4000, Permission: isla.MemoryRead},
			{VA: 0x400000, PA: 0x400000, Length: 0x1000, Permission: isla.MemoryRead},
			{VA: 0x3000, PA: 0x3000, Length: 0x1000, Permission: isla.MemoryReadWrite},
		},
		[]isla.MemoryBacking{
			{Address: 0x400000, Permission: isla.MemoryRead, Bytes: bytesIn},
			{Address: 0x3bf0, Permission: isla.MemoryReadWrite, Bytes: make([]byte, 80)},
		},
	)
}

func packetExpectedValue(t *testing.T, testCase packetCase) uint64 {
	if testCase.Status == "ok" {
		if len(testCase.Value) < 3 || testCase.Value[:2] != "0x" {
			t.Fatalf("case %q expected value %q lacks lowercase 0x prefix", testCase.Name, testCase.Value)
		}
		value, err := strconv.ParseUint(testCase.Value[2:], 16, 64)
		if err != nil || testCase.ErrorCode != 0 {
			t.Fatalf("case %q expected value %q: %v", testCase.Name, testCase.Value, err)
		}
		return value
	}
	if testCase.Status != "error" || testCase.Value != "" || testCase.ErrorCode < 1 || testCase.ErrorCode > 8 {
		t.Fatalf("case %q has malformed error expectation: %#v", testCase.Name, testCase)
	}
	return uint64(1)<<63 | testCase.ErrorCode
}
