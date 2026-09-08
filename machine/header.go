// This file validates the fixed ELF container and executable profile.
// It must not validate RISC-V machine flags.
package machine

import (
	"debug/elf"
	"fmt"
)

func validateHeader(file *elf.File, content []byte) (uint32, error) {
	if err := validateELFProfile(file); err != nil {
		return 0, err
	}
	return validateRISCVHeader(file, content)
}

func validateELFProfile(file *elf.File) error {
	if file.Class != elf.ELFCLASS64 {
		return reject(UnsupportedELFClass, file.Class.String(), nil)
	}
	if file.Data != elf.ELFDATA2LSB {
		return reject(UnsupportedELFData, file.Data.String(), nil)
	}
	if file.OSABI != elf.ELFOSABI_NONE && file.OSABI != elf.ELFOSABI_LINUX {
		return reject(UnsupportedELFOSABI, file.OSABI.String(), nil)
	}
	if file.ABIVersion != 0 {
		return reject(UnsupportedELFABIVersion, fmt.Sprint(file.ABIVersion), nil)
	}
	if file.Type != elf.ET_EXEC {
		return reject(UnsupportedELFType, file.Type.String(), nil)
	}
	return nil
}
