// These tests pin public ELF header precedence and exact rejection details.
// They must use repository ELF fixtures through machine.Load.
package machine_test

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestLoadPreservesMixedHeaderPrecedence(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	content[7] = byte(elf.ELFOSABI_FREEBSD)
	content[8] = 1
	binary.LittleEndian.PutUint16(content[elfTypeOffset:elfTypeOffset+2], uint16(elf.ET_DYN))
	binary.LittleEndian.PutUint16(content[elfMachineOffset:elfMachineOffset+2], uint16(elf.EM_AARCH64))
	binary.LittleEndian.PutUint32(content[elfFlagsOffset:elfFlagsOffset+4], unknownLoadFlag)

	rejection := requireRejection(t, content, uint64(len(content)), machine.UnsupportedELFOSABI)
	if rejection.Detail != elf.ELFOSABI_FREEBSD.String() {
		t.Fatalf("rejection detail = %q, want %q", rejection.Detail, elf.ELFOSABI_FREEBSD.String())
	}
}

func TestLoadRejectsWrongArchitectureBeforeInvalidRISCVFlags(t *testing.T) {
	cases := []struct {
		name    string
		machine elf.Machine
	}{
		{"AARCH64", elf.EM_AARCH64},
		{"X86_64", elf.EM_X86_64},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			content := fixture(t, "rv64-lp64d-static.elf")
			binary.LittleEndian.PutUint16(content[elfMachineOffset:elfMachineOffset+2], uint16(test.machine))
			binary.LittleEndian.PutUint32(content[elfFlagsOffset:elfFlagsOffset+4], unknownLoadFlag)

			rejection := requireRejection(t, content, uint64(len(content)), machine.UnsupportedELFMachine)
			wantDetail := fmt.Sprint(test.machine)
			if rejection.Detail != wantDetail {
				t.Fatalf("rejection detail = %q, want %q", rejection.Detail, wantDetail)
			}
		})
	}
}
