// Extent tests keep a proof to the code it names and the code that code
// calls. Proving the whole file, or failing to follow a call, are both wrong.
package isla

import (
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func wordAt(address uint64, word uint32) machine.Instruction {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, word)
	return machine.Instruction{Address: address, Bytes: bytes}
}

// callWord builds an A64 BL from one address to another.
func callWord(from uint64, to uint64) uint32 {
	offset := (int64(to) - int64(from)) / 4
	return 0x94000000 | uint32(offset)&0x03ffffff
}

const returnWord uint32 = 0xd65f03c0
const nopWord uint32 = 0xd503201f

func TestReachableInstructionsFollowsACall(t *testing.T) {
	image := []machine.Instruction{
		wordAt(0x1000, nopWord),
		wordAt(0x1004, callWord(0x1004, 0x2000)),
		wordAt(0x1008, returnWord),
		wordAt(0x1500, nopWord),
		wordAt(0x2000, returnWord),
	}
	reached := reachableInstructions(image, 0x1000, 0x100c)
	addresses := make([]uint64, 0, len(reached))
	for _, instruction := range reached {
		addresses = append(addresses, instruction.Address)
	}
	want := []uint64{0x1000, 0x1004, 0x1008, 0x2000}
	if len(addresses) != len(want) {
		t.Fatalf("reachableInstructions() returned %v, want %v", addresses, want)
	}
	for index := range want {
		if addresses[index] != want[index] {
			t.Errorf("address %d = %#x, want %#x", index, addresses[index], want[index])
		}
	}
}

func TestReachableInstructionsLeavesUncalledCodeOut(t *testing.T) {
	image := []machine.Instruction{
		wordAt(0x1000, returnWord),
		wordAt(0x9000, nopWord),
	}
	reached := reachableInstructions(image, 0x1000, 0x1004)
	if len(reached) != 1 || reached[0].Address != 0x1000 {
		t.Fatalf("reachableInstructions() returned %d instructions, want only 0x1000", len(reached))
	}
}

func TestReachableInstructionsStopsOnACycle(t *testing.T) {
	image := []machine.Instruction{
		wordAt(0x1000, callWord(0x1000, 0x2000)),
		wordAt(0x1004, returnWord),
		wordAt(0x2000, callWord(0x2000, 0x1000)),
		wordAt(0x2004, returnWord),
	}
	reached := reachableInstructions(image, 0x1000, 0x1008)
	if len(reached) != 4 {
		t.Fatalf("reachableInstructions() returned %d instructions, want 4", len(reached))
	}
}

func TestCallTargetReadsABranchWithLink(t *testing.T) {
	target, calls := callTarget(wordAt(0x1004, callWord(0x1004, 0x2000)))
	if !calls || target != 0x2000 {
		t.Errorf("callTarget() = %#x, %v, want 0x2000, true", target, calls)
	}
	if _, calls := callTarget(wordAt(0x1000, nopWord)); calls {
		t.Error("callTarget() read a call from a nop")
	}
}
