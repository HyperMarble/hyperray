// This file applies ARM64 Mach-O segment admission rules.
// It must reject invalid PAGEZERO and incomplete header-bearing TEXT segments.
package arm64

import (
	"debug/macho"
	"fmt"
	"math"
)

func validateSpecialSegment(header macho.SegmentHeader, index int) error {
	if header.Name != "__PAGEZERO" {
		return nil
	}
	if header.Filesz != 0 || header.Nsect != 0 || header.Prot != 0 || header.Maxprot != 0 {
		return &Rejection{Code: InvalidPagezero, Detail: fmt.Sprintf("segment[%d] __PAGEZERO is not empty and inaccessible", index)}
	}
	return nil
}

func validateMaterializedSize(segments []mappedSegment, maximum uint64) error {
	var total uint64
	for _, segment := range segments {
		if segment.header.Memsz > math.MaxUint64-total {
			return &Rejection{Code: LoadedByteCapacityExceeded, Detail: "materialized memory size overflows uint64"}
		}
		total += segment.header.Memsz
	}
	if total > maximum {
		return &Rejection{Code: LoadedByteCapacityExceeded, Detail: fmt.Sprintf("materialized memory size=%d exceeds capacity=%d", total, maximum)}
	}
	return nil
}

func validateTextHeader(headers []macho.SegmentHeader, content []byte) error {
	count := 0
	order := commandTableByteOrder(content)
	header := decodeMachHeader(content, order)
	needed := uint64(machHeader64Size) + uint64(header.Cmdsz)
	for _, segment := range headers {
		if segment.Name != "__TEXT" {
			continue
		}
		count++
		if segment.Offset != 0 || segment.Filesz < needed {
			return &Rejection{Code: MissingTextSegment, Detail: "__TEXT does not contain the Mach-O header and command table"}
		}
	}
	if count != 1 {
		return &Rejection{Code: MissingTextSegment, Detail: fmt.Sprintf("found %d __TEXT segments", count)}
	}
	return nil
}
