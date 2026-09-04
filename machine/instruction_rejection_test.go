// These tests reject invalid executable regions and instruction framing.
// They must not assert instruction semantics.
package machine_test

import (
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestRejectsMissingAndUnalignedCode(t *testing.T) {
	content := fixture(t, "rv64-lp64d-no-code.elf")
	requireRejection(t, content, uint64(len(content)), machine.MissingExecutableRegion)
	content = fixture(t, "rv64-lp64d-static.elf")
	setProgramUint64(content, 0, programAlignmentOffset, 1)
	address := programUint64(content, 0, programAddressOffset)
	setProgramUint64(content, 0, programAddressOffset, address+1)
	requireRejection(t, content, uint64(len(content)), machine.UnalignedExecutableRegion)
}

func TestRejectsInvalidInstructionFraming(t *testing.T) {
	cases := []struct {
		name string
		code machine.RejectionCode
	}{
		{"rv64-lp64d-odd.elf", machine.TruncatedInstructionEncoding},
		{"rv64-lp64d-truncated.elf", machine.TruncatedInstructionEncoding},
		{"rv64-lp64d-long.elf", machine.UnsupportedInstructionLength},
	}
	for _, test := range cases {
		content := fixture(t, test.name)
		requireRejection(t, content, uint64(len(content)), test.code)
	}
}

func TestRejectsEntryBetweenInstructions(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	entry := binary.LittleEndian.Uint64(content[elfEntryOffset : elfEntryOffset+8])
	binary.LittleEndian.PutUint64(content[elfEntryOffset:elfEntryOffset+8], entry+instructionParcelSize)
	requireRejection(t, content, uint64(len(content)), machine.EntryNotInstruction)
}
