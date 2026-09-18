// A proof loads the pages it can reach. A page it cannot reach is absent,
// and reading it is refused rather than answered with zeros.
package arm64_test

import (
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func call(from uint64, to uint64) machine.Instruction {
	offset := (int64(to) - int64(from)) / 4
	word := uint32(0x25)<<26 | uint32(offset)&0x03FFFFFF
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, word)
	return machine.Instruction{Address: from, Bytes: bytes}
}

func TestTheFunctionPageIsReachable(t *testing.T) {
	pages := arm64.ReachablePages(nil, 0x100000000, 0x100000018)
	if !arm64.PageCovers(pages, 0x100000000) {
		t.Error("the function page is absent")
	}
}

func TestACalledPageIsReachable(t *testing.T) {
	calls := []machine.Instruction{call(0x100000000, 0x100005000)}
	pages := arm64.ReachablePages(calls, 0x100000000, 0x100000018)
	if !arm64.PageCovers(pages, 0x100005000) {
		t.Error("a called page is absent")
	}
}

func TestAPageNothingReachesIsAbsent(t *testing.T) {
	pages := arm64.ReachablePages(nil, 0x100000000, 0x100000018)
	if arm64.PageCovers(pages, 0x100044000) {
		t.Error("an unreached page was included")
	}
}

func TestACallOutsideTheRangeIsNotFollowed(t *testing.T) {
	calls := []machine.Instruction{call(0x100009000, 0x100005000)}
	pages := arm64.ReachablePages(calls, 0x100000000, 0x100000018)
	if arm64.PageCovers(pages, 0x100005000) {
		t.Error("a call outside the stated range was followed")
	}
}

func TestAPageNamedByAdrpIsReachable(t *testing.T) {
	// Real output: adrp x8, 0x100000000 at 0x10000033c, which the objdump
	// listing of f_struct_small.bin states.
	adrp := machine.Instruction{Address: 0x10000033c, Bytes: []byte{0x08, 0x00, 0x00, 0x90}}
	pages := arm64.ReachablePages([]machine.Instruction{adrp}, 0x100000338, 0x100000360)
	if !arm64.PageCovers(pages, 0x100000370) {
		t.Fatal("the page an ADRP names is absent")
	}
}

func TestThePageAfterAnAdrpIsReachable(t *testing.T) {
	// An offset on the load can carry past the page ADRP named.
	adrp := machine.Instruction{Address: 0x10000033c, Bytes: []byte{0x08, 0x00, 0x00, 0x90}}
	pages := arm64.ReachablePages([]machine.Instruction{adrp}, 0x100000338, 0x100000360)
	if !arm64.PageCovers(pages, 0x100001000) {
		t.Fatal("the page after an ADRP is absent")
	}
}
