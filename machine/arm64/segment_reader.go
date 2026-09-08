// This file reads bounded LC_SEGMENT_64 metadata from an ARM64 Mach-O.
// It does not admit ignored runtime obligations or validate other payloads.
package arm64

import (
	"debug/macho"
	"encoding/binary"
	"fmt"
)

// ReadSegmentHeaders returns ordered, owned LC_SEGMENT_64 metadata.
// Other commands remain uninterpreted. Empty metadata does not claim loadability.
func ReadSegmentHeaders(content []byte) ([]macho.SegmentHeader, error) {
	if err := ValidateCommandTable(content); err != nil {
		return nil, err
	}
	order := commandTableByteOrder(content)
	header := decodeMachHeader(content, order)
	table := content[machHeader64Size : machHeader64Size+int(header.Cmdsz)]
	segments := make([]macho.SegmentHeader, 0)
	cursor := uint64(0)
	for commandIndex := uint32(0); commandIndex < header.Ncmd; commandIndex++ {
		command := table[int(cursor):]
		commandSize := uint64(order.Uint32(command[4:8]))
		fileOffset := uint64(machHeader64Size) + cursor
		commandID := macho.LoadCmd(order.Uint32(command[0:4]))
		if commandID == macho.LoadCmdSegment {
			return nil, &Rejection{Code: UnsupportedSegmentCommand, Detail: fmt.Sprintf("command[%d] fileoffset=%d: LC_SEGMENT is unsupported", commandIndex, fileOffset)}
		}
		if commandID != macho.LoadCmdSegment64 {
			cursor += commandSize
			continue
		}
		if err := validateSegmentCommandShape(command, commandIndex, fileOffset, order); err != nil {
			return nil, err
		}
		segment := decodeSegmentHeader(command, commandSize, order)
		if err := ValidateSegmentRange(segment, uint64(len(content))); err != nil {
			return nil, err
		}
		segments = append(segments, segment)
		cursor += commandSize
	}
	return segments, nil
}

func decodeSegmentHeader(command []byte, commandSize uint64, order binary.ByteOrder) macho.SegmentHeader {
	return macho.SegmentHeader{
		Cmd:     macho.LoadCmd(order.Uint32(command[0:4])),
		Len:     uint32(commandSize),
		Name:    decodeSegmentName(command[8:24]),
		Addr:    order.Uint64(command[24:32]),
		Memsz:   order.Uint64(command[32:40]),
		Offset:  order.Uint64(command[40:48]),
		Filesz:  order.Uint64(command[48:56]),
		Maxprot: order.Uint32(command[56:60]),
		Prot:    order.Uint32(command[60:64]),
		Nsect:   order.Uint32(command[64:68]),
		Flag:    order.Uint32(command[68:72]),
	}
}

func decodeSegmentName(raw []byte) string {
	for index, value := range raw {
		if value == 0 {
			return string(raw[:index])
		}
	}
	return string(raw)
}
