// This file validates one load segment without loading its bytes.
// It must reject every size, range, alignment, and permission conflict.
package machine

import (
	"debug/elf"
	"fmt"
	"math"
)

const acceptedLoadFlags = elf.PF_R | elf.PF_W | elf.PF_X

func validateSegment(header elf.ProgHeader, content []byte, index int) error {
	detail := fmt.Sprintf("program header %d", index)
	if header.Flags&^acceptedLoadFlags != 0 {
		return reject(InvalidLoadFlags, fmt.Sprintf("%s has flags %#x", detail, header.Flags), nil)
	}
	if header.Filesz > header.Memsz {
		return reject(FileSizeExceedsMemorySize, fmt.Sprintf("%s has filesz %d and memsz %d", detail, header.Filesz, header.Memsz), nil)
	}
	if header.Memsz > math.MaxUint64-header.Vaddr {
		return reject(SegmentAddressOverflow, fmt.Sprintf("%s has vaddr %#x and memsz %d", detail, header.Vaddr, header.Memsz), nil)
	}
	contentSize := uint64(len(content))
	if header.Off > contentSize || header.Filesz > contentSize-header.Off {
		return reject(SegmentOutsideArtifact, fmt.Sprintf("%s has offset %d, filesz %d, and artifact size %d", detail, header.Off, header.Filesz, contentSize), nil)
	}
	if !validAlignment(header) {
		return reject(InvalidSegmentAlignment, fmt.Sprintf("%s has align %d, vaddr %#x, and offset %d", detail, header.Align, header.Vaddr, header.Off), nil)
	}
	if header.Flags&elf.PF_X != 0 && header.Flags&elf.PF_W != 0 {
		return reject(WritableExecutableSegment, fmt.Sprintf("%s has flags %#x", detail, header.Flags), nil)
	}
	return nil
}

func validAlignment(header elf.ProgHeader) bool {
	if header.Align <= 1 {
		return true
	}
	if header.Align&(header.Align-1) != 0 {
		return false
	}
	return header.Vaddr%header.Align == header.Off%header.Align
}

func validateSegmentOrder(segments []loadSegment) error {
	for index := 1; index < len(segments); index++ {
		previousEnd := segments[index-1].header.Vaddr + segments[index-1].header.Memsz
		if previousEnd > segments[index].header.Vaddr {
			detail := fmt.Sprintf("previous end %#x exceeds sorted segment %d start %#x", previousEnd, index, segments[index].header.Vaddr)
			return reject(OverlappingLoadSegments, detail, nil)
		}
	}
	return nil
}
