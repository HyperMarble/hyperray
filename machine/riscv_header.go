// This file validates RISC-V machine identity and the LP64D ELF flags.
// It must not infer instruction-set coverage from ELF flags.
package machine

import (
	"debug/elf"
	"encoding/binary"
	"fmt"

	"github.com/HyperMarble/hyperray/machine/riscv"
)

const (
	elf64FlagsOffset = 48
)

func validateRISCVHeader(file *elf.File, content []byte) (uint32, error) {
	if !riscv.AcceptsMachine(file.Machine) {
		return 0, reject(UnsupportedELFMachine, file.Machine.String(), nil)
	}
	flags := binary.LittleEndian.Uint32(content[elf64FlagsOffset : elf64FlagsOffset+4])
	if !riscv.AcceptsFlags(flags) {
		return 0, reject(UnsupportedRISCVFlags, fmt.Sprintf("0x%x", flags), nil)
	}
	return flags, nil
}
