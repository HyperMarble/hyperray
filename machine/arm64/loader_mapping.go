// This file materializes admitted Mach-O segment memory.
// It must preserve file bytes, zero-fill, and segment permissions.
package arm64

import "github.com/HyperMarble/hyperray/machine"

func materializeSegments(content []byte, segments []mappedSegment, blanks []ZeroFillRange, pages []uint64) []machine.LoadedByte {
	loaded := make([]machine.LoadedByte, 0)
	for _, segment := range segments {
		memory := make([]machine.LoadedByte, 0, int(segment.header.Memsz))
		for index := 0; index < int(segment.header.Memsz); index++ {
			address := segment.header.Addr + uint64(index)
			if len(pages) != 0 && !PageCovers(pages, address) {
				continue
			}
			value := byte(0)
			if uint64(index) < segment.header.Filesz && !anyCovers(blanks, address) {
				value = content[int(segment.header.Offset)+index]
			}
			memory = append(memory, machine.LoadedByte{Address: address, Value: value, Permissions: segment.permissions})
		}
		loaded = append(loaded, memory...)
	}
	return loaded
}

func anyCovers(blanks []ZeroFillRange, address uint64) bool {
	for _, blank := range blanks {
		if blank.Covers(address) {
			return true
		}
	}
	return false
}

func executableSegmentRegions(segments []mappedSegment) []machine.ExecutableRegion {
	regions := make([]machine.ExecutableRegion, 0)
	for _, segment := range segments {
		if !segment.permissions.Executable {
			continue
		}
		regions = append(regions, machine.ExecutableRegion{StartAddress: segment.header.Addr, ByteLength: segment.header.Memsz})
	}
	return regions
}
