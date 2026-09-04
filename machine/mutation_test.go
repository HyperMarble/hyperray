// These functions change named ELF64 header fields for adversarial tests.
// They must not depend on one fixture file offset outside the ELF format.
package machine_test

import "encoding/binary"

const (
	elfTypeOffset            = 16
	elfMachineOffset         = 18
	elfEntryOffset           = 24
	elfProgramOffset         = 32
	elfFlagsOffset           = 48
	elfProgramSizeOffset     = 54
	programTypeOffset        = 0
	programFlagsOffset       = 4
	programFileOffset        = 8
	programAddressOffset     = 16
	programFileSizeOffset    = 32
	programMemorySizeOffset  = 40
	programAlignmentOffset   = 48
	riscvTotalStoreOrderFlag = 0x10
	riscvRV64ILP32Flag       = 0x20
	riscvDoubleFloatFlag     = 0x4
	unknownLoadFlag          = 0x8
	instructionParcelSize    = 2
	nonPowerOfTwoAlignment   = 3
)

func programOffset(content []byte, index int) int {
	start := binary.LittleEndian.Uint64(content[elfProgramOffset : elfProgramOffset+8])
	size := binary.LittleEndian.Uint16(content[elfProgramSizeOffset : elfProgramSizeOffset+2])
	return int(start) + index*int(size)
}

func setProgramUint32(content []byte, index int, field int, value uint32) {
	offset := programOffset(content, index) + field
	binary.LittleEndian.PutUint32(content[offset:offset+4], value)
}

func setProgramUint64(content []byte, index int, field int, value uint64) {
	offset := programOffset(content, index) + field
	binary.LittleEndian.PutUint64(content[offset:offset+8], value)
}

func programUint64(content []byte, index int, field int) uint64 {
	offset := programOffset(content, index) + field
	return binary.LittleEndian.Uint64(content[offset : offset+8])
}

func elfFlags(content []byte) uint32 {
	return binary.LittleEndian.Uint32(content[elfFlagsOffset : elfFlagsOffset+4])
}

func swapPrograms(content []byte, left int, right int) {
	size := int(binary.LittleEndian.Uint16(content[elfProgramSizeOffset : elfProgramSizeOffset+2]))
	leftOffset := programOffset(content, left)
	rightOffset := programOffset(content, right)
	saved := append([]byte(nil), content[leftOffset:leftOffset+size]...)
	copy(content[leftOffset:leftOffset+size], content[rightOffset:rightOffset+size])
	copy(content[rightOffset:rightOffset+size], saved)
}
