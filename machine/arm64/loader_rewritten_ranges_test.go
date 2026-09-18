// A function that never reaches an address the loader rewrites can be proved
// from the bytes in the file. One that does reach such an address cannot,
// because those bytes change before the program runs.
package arm64_test

import (
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func branchLink(from uint64, to uint64) machine.Instruction {
	offset := (int64(to) - int64(from)) / 4
	word := uint32(0x25)<<26 | uint32(offset)&0x03FFFFFF
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, word)
	return machine.Instruction{Address: from, Bytes: bytes}
}

func TestACallIntoARewrittenRangeIsReported(t *testing.T) {
	ranges := []arm64.RewrittenRange{{Address: 0x4000, Length: 0x100}}
	calls := []machine.Instruction{branchLink(0x1000, 0x4010)}
	if err := arm64.ReadsRewrittenAddress(calls, ranges); err == nil {
		t.Fatal("ReadsRewrittenAddress() accepted a call into a rewritten range")
	}
}

func TestACallOutsideEveryRewrittenRangeIsAccepted(t *testing.T) {
	ranges := []arm64.RewrittenRange{{Address: 0x4000, Length: 0x100}}
	calls := []machine.Instruction{branchLink(0x1000, 0x2000)}
	if err := arm64.ReadsRewrittenAddress(calls, ranges); err != nil {
		t.Fatalf("ReadsRewrittenAddress() error = %v", err)
	}
}

func TestAnInstructionThatIsNotACallIsIgnored(t *testing.T) {
	ranges := []arm64.RewrittenRange{{Address: 0, Length: 0xFFFFFFFF}}
	moves := []machine.Instruction{{Address: 0x1000, Bytes: []byte{0x40, 0x05, 0x80, 0xd2}}}
	if err := arm64.ReadsRewrittenAddress(moves, ranges); err != nil {
		t.Fatalf("ReadsRewrittenAddress() error = %v", err)
	}
}

func TestNoRangesMeansNothingIsRewritten(t *testing.T) {
	calls := []machine.Instruction{branchLink(0x1000, 0x4010)}
	if err := arm64.ReadsRewrittenAddress(calls, nil); err != nil {
		t.Fatalf("ReadsRewrittenAddress() error = %v", err)
	}
}
