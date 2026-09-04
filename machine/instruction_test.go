// These tests make sure that instruction records partition executable bytes.
// They must not assign opcode names or effects.
package machine_test

import (
	"bytes"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestInstructionsPartitionExecutableBytes(t *testing.T) {
	image, content := acceptedFixture(t, "rv64-lp64d-static.elf")
	executable := executableBytes(image.LoadedBytes)
	position := 0
	for _, instruction := range image.Instructions {
		position = compareInstruction(t, executable, instruction, position)
	}
	if position != len(executable) {
		t.Errorf("instructions cover %d executable bytes, want %d", position, len(executable))
	}
	if len(content) == 0 {
		t.Error("fixture is empty")
	}
}

func executableBytes(loaded []machine.LoadedByte) []machine.LoadedByte {
	result := make([]machine.LoadedByte, 0)
	for _, loadedByte := range loaded {
		if loadedByte.Permissions.Executable {
			result = append(result, loadedByte)
		}
	}
	return result
}

func compareInstruction(t *testing.T, executable []machine.LoadedByte, instruction machine.Instruction, start int) int {
	t.Helper()
	length := len(instruction.Bytes)
	if length != 2 && length != 4 {
		t.Fatalf("instruction at %#x has %d bytes", instruction.Address, length)
	}
	if start+length > len(executable) {
		t.Fatalf("instruction at %#x exceeds executable bytes", instruction.Address)
	}
	if instruction.Address != executable[start].Address {
		t.Errorf("instruction address = %#x, want %#x", instruction.Address, executable[start].Address)
	}
	wantBytes := make([]byte, length)
	for offset := range wantBytes {
		wantBytes[offset] = executable[start+offset].Value
	}
	if !bytes.Equal(instruction.Bytes, wantBytes) {
		t.Errorf("instruction bytes = %x, want %x", instruction.Bytes, wantBytes)
	}
	return start + length
}

func TestLoadSortsProgramHeadersByAddress(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	swapPrograms(content, 0, 1)
	image, err := machine.Load(content, uint64(len(content)))
	if err != nil {
		t.Fatalf("machine.Load() error = %v", err)
	}
	if len(image.LoadedBytes) == 0 || len(image.ExecutableRegions) == 0 {
		t.Fatal("accepted image has no executable bytes")
	}
	if image.LoadedBytes[0].Address != image.ExecutableRegions[0].StartAddress {
		t.Errorf("first loaded address = %#x, executable start = %#x", image.LoadedBytes[0].Address, image.ExecutableRegions[0].StartAddress)
	}
}
