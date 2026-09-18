// These helpers build literal expected public metadata records.
// They must not call the section reader or validate synthetic input.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"strings"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func sectionRecord(commandIndex, sectionIndex uint32, command []byte, value sectionFixture) arm64.SectionRecord {
	return arm64.SectionRecord{
		CommandIndex: commandIndex,
		SectionIndex: sectionIndex,
		Segment:      segmentHeader(command),
		Header:       sectionHeader(value),
		Reserved1:    value.reserved1,
		Reserved2:    value.reserved2,
		Reserved3:    value.reserved3,
	}
}

func segmentHeader(command []byte) macho.SegmentHeader {
	return macho.SegmentHeader{
		Cmd: macho.LoadCmdSegment64, Len: uint32(len(command)),
		Name: nulTerminatedName(command[8:24]), Addr: binary.LittleEndian.Uint64(command[24:32]),
		Memsz: binary.LittleEndian.Uint64(command[32:40]), Offset: binary.LittleEndian.Uint64(command[40:48]),
		Filesz: binary.LittleEndian.Uint64(command[48:56]), Maxprot: binary.LittleEndian.Uint32(command[56:60]),
		Prot: binary.LittleEndian.Uint32(command[60:64]), Nsect: binary.LittleEndian.Uint32(command[64:68]),
		Flag: binary.LittleEndian.Uint32(command[68:72]),
	}
}

func sectionHeader(value sectionFixture) macho.SectionHeader {
	return macho.SectionHeader{Name: nulTerminatedName([]byte(value.name)), Seg: nulTerminatedName([]byte(value.segment)), Addr: value.addr, Size: value.size, Offset: value.offset, Align: value.align, Reloff: value.reloff, Nreloc: value.nreloc, Flags: value.flags}
}

func nulTerminatedName(value []byte) string {
	if index := strings.IndexByte(string(value), 0); index >= 0 {
		return string(value[:index])
	}
	return string(value)
}

func hasFileBackedCodeSection(records []arm64.SectionRecord) bool {
	for _, record := range records {
		if record.Header.Name == "__text" && record.Header.Offset != 0 && record.Header.Size != 0 {
			return true
		}
	}
	return false
}
