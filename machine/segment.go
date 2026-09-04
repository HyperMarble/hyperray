// This file converts PT_LOAD headers into bounded memory segments.
// It must reject ambiguous, dynamic, or writable code images.
package machine

import (
	"debug/elf"
	"fmt"
	"sort"
)

type loadSegment struct {
	header elf.ProgHeader
	data   []byte
}

func collectSegments(file *elf.File, content []byte, maximum uint64) ([]loadSegment, error) {
	segments := make([]loadSegment, 0)
	var total uint64
	for index, program := range file.Progs {
		if program.Type == elf.PT_INTERP || program.Type == elf.PT_DYNAMIC {
			detail := fmt.Sprintf("program header %d has type %s", index, program.Type)
			return nil, reject(DynamicExecutable, detail, nil)
		}
		if program.Type != elf.PT_LOAD {
			continue
		}
		if err := validateSegment(program.ProgHeader, content, index); err != nil {
			return nil, err
		}
		if program.Memsz > maximum-total {
			detail := fmt.Sprintf("program header %d needs %d bytes after %d of %d", index, program.Memsz, total, maximum)
			return nil, reject(LoadedByteCapacityExceeded, detail, nil)
		}
		total += program.Memsz
		segments = append(segments, loadSegment{header: program.ProgHeader})
	}
	if total == 0 {
		return nil, reject(MissingLoadableBytes, "no PT_LOAD memory bytes", nil)
	}
	sort.Slice(segments, func(left, right int) bool {
		return segments[left].header.Vaddr < segments[right].header.Vaddr
	})
	if err := validateSegmentOrder(segments); err != nil {
		return nil, err
	}
	return materializeSegments(segments, content), nil
}
