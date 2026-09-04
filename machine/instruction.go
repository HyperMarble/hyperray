// This file frames executable bytes by the RISC-V length encoding.
// It must not decode an opcode or assign an instruction effect.
package machine

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
)

const instructionParcelByteLength = 2
const standardInstructionByteLength = 4
const compressedEncodingMask = 0x3
const standardEncodingMask = 0x1f

func recordInstructions(segments []loadSegment) ([]ExecutableRegion, []Instruction, error) {
	regions := make([]ExecutableRegion, 0)
	instructions := make([]Instruction, 0)
	for _, segment := range segments {
		if segment.header.Flags&elf.PF_X == 0 {
			continue
		}
		if segment.header.Vaddr%instructionParcelByteLength != 0 {
			return nil, nil, reject(UnalignedExecutableRegion, fmt.Sprintf("0x%x", segment.header.Vaddr), nil)
		}
		region := ExecutableRegion{StartAddress: segment.header.Vaddr, ByteLength: segment.header.Memsz}
		regions = append(regions, region)
		framed, err := frameRegion(segment)
		if err != nil {
			return nil, nil, err
		}
		instructions = append(instructions, framed...)
	}
	if len(regions) == 0 {
		return nil, nil, reject(MissingExecutableRegion, "no executable PT_LOAD segment", nil)
	}
	return regions, instructions, nil
}

func frameRegion(segment loadSegment) ([]Instruction, error) {
	result := make([]Instruction, 0)
	for offset := 0; offset < len(segment.data); {
		remaining := len(segment.data) - offset
		address := segment.header.Vaddr + uint64(offset)
		if remaining < instructionParcelByteLength {
			detail := fmt.Sprintf("address %#x has %d of %d parcel bytes", address, remaining, instructionParcelByteLength)
			return nil, reject(TruncatedInstructionEncoding, detail, nil)
		}
		parcel := binary.LittleEndian.Uint16(segment.data[offset : offset+instructionParcelByteLength])
		length := instructionLength(parcel)
		if length == 0 {
			detail := fmt.Sprintf("address %#x starts with parcel %#04x", address, parcel)
			return nil, reject(UnsupportedInstructionLength, detail, nil)
		}
		if remaining < length {
			detail := fmt.Sprintf("address %#x has %d of %d encoding bytes", address, remaining, length)
			return nil, reject(TruncatedInstructionEncoding, detail, nil)
		}
		encoding := append([]byte(nil), segment.data[offset:offset+length]...)
		result = append(result, Instruction{Address: address, Bytes: encoding})
		offset += length
	}
	return result, nil
}

func instructionLength(firstParcel uint16) int {
	if firstParcel&compressedEncodingMask != compressedEncodingMask {
		return instructionParcelByteLength
	}
	if firstParcel&standardEncodingMask != standardEncodingMask {
		return standardInstructionByteLength
	}
	return 0
}
