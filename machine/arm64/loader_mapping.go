// This file materializes admitted Mach-O segment memory.
// It must preserve file bytes, zero-fill, and segment permissions.
package arm64

import "github.com/HyperMarble/hyperray/machine"

func materializeSegments(content []byte, segments []mappedSegment, blanks []ZeroFillRange) []machine.LoadedByte {
	loaded := make([]machine.LoadedByte, 0)
	for _, segment := range segments {
		memory := make([]machine.LoadedByte, int(segment.header.Memsz))
		for index := range memory {
			address := segment.header.Addr + uint64(index)
			value := byte(0)
			if uint64(index) < segment.header.Filesz && !anyCovers(blanks, address) {
				value = content[int(segment.header.Offset)+index]
			}
			memory[index] = machine.LoadedByte{Address: address, Value: value, Permissions: segment.permissions}
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
