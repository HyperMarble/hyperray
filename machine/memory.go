// This file materializes and records each loadable memory byte.
// It must preserve file bytes, zero-fill, permissions, and address order.
package machine

import "debug/elf"

func materializeSegments(segments []loadSegment, content []byte) []loadSegment {
	for index := range segments {
		header := segments[index].header
		segments[index].data = make([]byte, int(header.Memsz))
		fileStart := int(header.Off)
		fileEnd := int(header.Off + header.Filesz)
		copy(segments[index].data, content[fileStart:fileEnd])
	}
	return segments
}

func recordLoadedBytes(segments []loadSegment) []LoadedByte {
	total := 0
	for _, segment := range segments {
		total += len(segment.data)
	}
	loaded := make([]LoadedByte, 0, total)
	for _, segment := range segments {
		permissions := segmentPermissions(segment.header.Flags)
		for offset, value := range segment.data {
			loaded = append(loaded, LoadedByte{
				Address:     segment.header.Vaddr + uint64(offset),
				Value:       value,
				Permissions: permissions,
			})
		}
	}
	return loaded
}

func segmentPermissions(flags elf.ProgFlag) Permissions {
	return Permissions{
		Readable:   flags&elf.PF_R != 0,
		Writable:   flags&elf.PF_W != 0,
		Executable: flags&elf.PF_X != 0,
	}
}
