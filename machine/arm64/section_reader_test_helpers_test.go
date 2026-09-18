// These helpers build complete ARM64 section records for external tests.
// They must not reproduce production validation or allocate from encoded counts.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
)

type sectionFixture struct {
	name, segment string
	addr, size    uint64
	offset        uint32
	align         uint32
	reloff        uint32
	nreloc        uint32
	flags         uint32
	reserved1     uint32
	reserved2     uint32
	reserved3     uint32
}

func sectionCommand(segmentName string, values ...sectionFixture) []byte {
	commandSize := uint32(72 + 80*len(values))
	command := syntheticSegment(macho.LoadCmdSegment64, commandSize, segmentName, uint32(len(values)))
	for index, value := range values {
		writeSectionFixture(command[72+80*index:], value)
	}
	return command
}

func writeSectionFixture(record []byte, value sectionFixture) {
	writeSectionName(record[0:16], value.name)
	writeSectionName(record[16:32], value.segment)
	binary.LittleEndian.PutUint64(record[32:40], value.addr)
	binary.LittleEndian.PutUint64(record[40:48], value.size)
	binary.LittleEndian.PutUint32(record[48:52], value.offset)
	binary.LittleEndian.PutUint32(record[52:56], value.align)
	binary.LittleEndian.PutUint32(record[56:60], value.reloff)
	binary.LittleEndian.PutUint32(record[60:64], value.nreloc)
	binary.LittleEndian.PutUint32(record[64:68], value.flags)
	binary.LittleEndian.PutUint32(record[68:72], value.reserved1)
	binary.LittleEndian.PutUint32(record[72:76], value.reserved2)
	binary.LittleEndian.PutUint32(record[76:80], value.reserved3)
}

func writeSectionName(destination []byte, name string) {
	copy(destination, []byte(name))
}
