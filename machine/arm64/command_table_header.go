// This file decodes only the fixed 64-bit Mach-O header fields.
// It must not inspect reserved words or interpret command payloads.
package arm64

import (
	"debug/macho"
	"encoding/binary"
)

func commandTableByteOrder(content []byte) binary.ByteOrder {
	if binary.LittleEndian.Uint32(content[0:4]) == macho.Magic64 {
		return binary.LittleEndian
	}
	if binary.BigEndian.Uint32(content[0:4]) == macho.Magic64 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func decodeMachHeader(content []byte, byteOrder binary.ByteOrder) macho.FileHeader {
	return macho.FileHeader{
		Magic:  byteOrder.Uint32(content[0:4]),
		Cpu:    macho.Cpu(byteOrder.Uint32(content[4:8])),
		SubCpu: byteOrder.Uint32(content[8:12]),
		Type:   macho.Type(byteOrder.Uint32(content[12:16])),
		Ncmd:   byteOrder.Uint32(content[16:20]),
		Cmdsz:  byteOrder.Uint32(content[20:24]),
		Flags:  byteOrder.Uint32(content[24:28]),
	}
}
