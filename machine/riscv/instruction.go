// This file returns the RISC-V instruction length from its first parcel.
// It must not decode an opcode or assign an instruction effect.
package riscv

// ParcelByteLength is the byte length of one RISC-V instruction parcel.
const ParcelByteLength = 2

const (
	standardInstructionByteLength = 4
	compressedEncodingMask        = 0x3
	standardEncodingMask          = 0x1f
)

// InstructionLength returns 2 or 4 bytes, or zero for an unsupported encoding.
func InstructionLength(firstParcel uint16) int {
	if firstParcel&compressedEncodingMask != compressedEncodingMask {
		return ParcelByteLength
	}
	if firstParcel&standardEncodingMask != standardEncodingMask {
		return standardInstructionByteLength
	}
	return 0
}
