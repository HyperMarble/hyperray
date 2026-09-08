// This file validates a thin ARM64 Mach-O executable header.
// It must not parse content, load memory, or imply dyld/runtime support.
package arm64

import (
	"debug/macho"
	"encoding/binary"
	"fmt"
)

// ValidateHeader accepts only the declared thin ARM64 Mach-O header profile.
func ValidateHeader(header macho.FileHeader, byteOrder binary.ByteOrder) error {
	if header.Magic != macho.Magic64 {
		return &Rejection{Code: UnsupportedMagic, Detail: fmt.Sprintf("0x%08x", header.Magic)}
	}
	if !isLittleEndian(byteOrder) {
		return &Rejection{Code: UnsupportedByteOrder, Detail: unsupportedByteOrderDetail(byteOrder)}
	}
	if header.Cpu != macho.CpuArm64 {
		return &Rejection{Code: UnsupportedCPU, Detail: fmt.Sprintf("%d", header.Cpu)}
	}
	if header.SubCpu != 0 {
		return &Rejection{Code: UnsupportedSubCPU, Detail: fmt.Sprintf("0x%08x", header.SubCpu)}
	}
	if header.Type != macho.TypeExec {
		return &Rejection{Code: UnsupportedType, Detail: fmt.Sprintf("%d", header.Type)}
	}
	// Header flags stay outside this policy. Load commands and fixups need later validation.
	return nil
}

func isLittleEndian(byteOrder binary.ByteOrder) bool {
	return byteOrder == binary.LittleEndian
}

func unsupportedByteOrderDetail(byteOrder binary.ByteOrder) string {
	if byteOrder == nil {
		return "nil"
	}
	if byteOrder == binary.BigEndian {
		return "big-endian"
	}
	return fmt.Sprintf("%T", byteOrder)
}
