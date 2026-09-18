// This file reads bounded ARM64 section metadata after segment validation.
// It must not classify sections or materialize relocation, section, or code bytes.
package arm64

import (
	"debug/macho"
	"encoding/binary"
)

// ReadSectionHeaders returns ordered section metadata with owning indices.
func ReadSectionHeaders(content []byte) ([]SectionRecord, error) {
	segments, err := ReadSegmentHeaders(content)
	if err != nil {
		return nil, err
	}
	order := commandTableByteOrder(content)
	header := decodeMachHeader(content, order)
	table := content[machHeader64Size : machHeader64Size+int(header.Cmdsz)]
	records := make([]SectionRecord, 0)
	segmentIndex := 0
	cursor := uint64(0)
	for commandIndex := uint32(0); commandIndex < header.Ncmd; commandIndex++ {
		command := table[int(cursor):]
		commandSize := uint64(order.Uint32(command[4:8]))
		if macho.LoadCmd(order.Uint32(command[0:4])) == macho.LoadCmdSegment64 {
			segment := segments[segmentIndex]
			records = append(records, decodeSectionRecords(command, segment, commandIndex, order)...)
			segmentIndex++
		}
		cursor += commandSize
	}
	return records, nil
}

func decodeSectionRecords(command []byte, segment macho.SegmentHeader, commandIndex uint32, order binary.ByteOrder) []SectionRecord {
	records := make([]SectionRecord, 0)
	sectionOffset := uint64(segmentCommand64Size)
	for sectionIndex := uint32(0); sectionIndex < segment.Nsect; sectionIndex++ {
		record := command[int(sectionOffset):]
		records = append(records, SectionRecord{
			CommandIndex: commandIndex,
			SectionIndex: sectionIndex,
			Segment:      segment,
			Header:       decodeSectionHeader(record, order),
			Reserved1:    order.Uint32(record[68:72]),
			Reserved2:    order.Uint32(record[72:76]),
			Reserved3:    order.Uint32(record[76:80]),
		})
		sectionOffset += section64Size
	}
	return records
}

func decodeSectionHeader(record []byte, order binary.ByteOrder) macho.SectionHeader {
	return macho.SectionHeader{
		Name:   decodeSegmentName(record[0:16]),
		Seg:    decodeSegmentName(record[16:32]),
		Addr:   order.Uint64(record[32:40]),
		Size:   order.Uint64(record[40:48]),
		Offset: order.Uint32(record[48:52]),
		Align:  order.Uint32(record[52:56]),
		Reloff: order.Uint32(record[56:60]),
		Nreloc: order.Uint32(record[60:64]),
		Flags:  order.Uint32(record[64:68]),
	}
}
