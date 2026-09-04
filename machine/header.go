// This file validates the fixed ELF and RISC-V ABI header profile.
// It must not infer instruction-set coverage from ELF flags.
package machine

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
)

const (
	elf64FlagsOffset       = 48
	riscvCompressedFlag    = 0x0001
	riscvFloatABIMask      = 0x0006
	riscvFloatABIDouble    = 0x0004
	riscvAcceptedFlagsMask = riscvCompressedFlag | riscvFloatABIMask
)

func validateHeader(file *elf.File, content []byte) (uint32, error) {
	if file.Class != elf.ELFCLASS64 {
		return 0, reject(UnsupportedELFClass, file.Class.String(), nil)
	}
	if file.Data != elf.ELFDATA2LSB {
		return 0, reject(UnsupportedELFData, file.Data.String(), nil)
	}
	if file.OSABI != elf.ELFOSABI_NONE && file.OSABI != elf.ELFOSABI_LINUX {
		return 0, reject(UnsupportedELFOSABI, file.OSABI.String(), nil)
	}
	if file.ABIVersion != 0 {
		return 0, reject(UnsupportedELFABIVersion, fmt.Sprint(file.ABIVersion), nil)
	}
	if file.Type != elf.ET_EXEC {
		return 0, reject(UnsupportedELFType, file.Type.String(), nil)
	}
	if file.Machine != elf.EM_RISCV {
		return 0, reject(UnsupportedELFMachine, file.Machine.String(), nil)
	}
	flags := binary.LittleEndian.Uint32(content[elf64FlagsOffset : elf64FlagsOffset+4])
	if flags&riscvFloatABIMask != riscvFloatABIDouble || flags&^riscvAcceptedFlagsMask != 0 {
		return 0, reject(UnsupportedRISCVFlags, fmt.Sprintf("0x%x", flags), nil)
	}
	return flags, nil
}
