// These tests reject ELF headers outside the declared public profile.
// They must use real ELF fixtures or named ELF header fields.
package machine_test

import (
	"debug/elf"
	"encoding/binary"
	"math"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestRejectsMalformedELFAndInvalidCapacity(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	rejection := requireRejection(t, []byte("not an ELF"), uint64(len(content)), machine.MalformedELF)
	if rejection.Cause == nil || rejection.Unwrap() == nil {
		t.Error("MalformedELF did not retain the debug/elf cause")
	}
	requireRejection(t, content, 0, machine.InvalidLoadCapacity)
	requireRejection(t, content, math.MaxUint64, machine.InvalidLoadCapacity)
}

func TestRejectsELFRepresentation(t *testing.T) {
	cases := []struct {
		name string
		code machine.RejectionCode
	}{
		{"rv32-ilp32d-static.elf", machine.UnsupportedELFClass},
		{"ppc64-big-static.elf", machine.UnsupportedELFData},
		{"rv64-lp64-static.elf", machine.UnsupportedRISCVFlags},
	}
	for _, test := range cases {
		content := fixture(t, test.name)
		requireRejection(t, content, uint64(len(content)), test.code)
	}
}

func TestRejectsELFIdentityFields(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	content[7] = byte(elf.ELFOSABI_FREEBSD)
	requireRejection(t, content, uint64(len(content)), machine.UnsupportedELFOSABI)
	content = fixture(t, "rv64-lp64d-static.elf")
	content[8] = 1
	requireRejection(t, content, uint64(len(content)), machine.UnsupportedELFABIVersion)
	content = fixture(t, "rv64-lp64d-static.elf")
	binary.LittleEndian.PutUint16(content[elfTypeOffset:elfTypeOffset+2], uint16(elf.ET_DYN))
	requireRejection(t, content, uint64(len(content)), machine.UnsupportedELFType)
	content = fixture(t, "rv64-lp64d-static.elf")
	binary.LittleEndian.PutUint16(content[elfMachineOffset:elfMachineOffset+2], uint16(elf.EM_X86_64))
	requireRejection(t, content, uint64(len(content)), machine.UnsupportedELFMachine)
}

func TestRejectsUnknownAndMissingRVCFlags(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	flags := elfFlags(content) | riscvRV64ILP32Flag
	binary.LittleEndian.PutUint32(content[elfFlagsOffset:elfFlagsOffset+4], flags)
	requireRejection(t, content, uint64(len(content)), machine.UnsupportedRISCVFlags)
	content = fixture(t, "rv64-lp64d-static.elf")
	flags = elfFlags(content) | riscvTotalStoreOrderFlag
	binary.LittleEndian.PutUint32(content[elfFlagsOffset:elfFlagsOffset+4], flags)
	requireRejection(t, content, uint64(len(content)), machine.UnsupportedRISCVFlags)
	content = fixture(t, "rv64-lp64d-static.elf")
	binary.LittleEndian.PutUint32(content[elfFlagsOffset:elfFlagsOffset+4], riscvDoubleFloatFlag)
	requireRejection(t, content, uint64(len(content)), machine.UnsupportedRISCVFlags)
}
