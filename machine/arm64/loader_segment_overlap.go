// This file rejects overlapping Mach-O virtual and file-backed ranges.
// It must preserve every admitted segment as a distinct mapping.
package arm64

import (
	"debug/macho"
	"fmt"
	"sort"
)

func validateSegmentLayout(headers []macho.SegmentHeader) error {
	ordered := append([]macho.SegmentHeader(nil), headers...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].Addr < ordered[right].Addr })
	for index := 1; index < len(ordered); index++ {
		previous := ordered[index-1]
		current := ordered[index]
		if previous.Memsz > current.Addr-previous.Addr {
			return &Rejection{Code: SegmentOverlap, Detail: fmt.Sprintf("segments %q and %q overlap", previous.Name, current.Name)}
		}
	}
	for left := range headers {
		for right := left + 1; right < len(headers); right++ {
			if fileRangesOverlap(headers[left], headers[right]) {
				return &Rejection{Code: FileBackedOverlap, Detail: fmt.Sprintf("segments %q and %q share file-backed bytes", headers[left].Name, headers[right].Name)}
			}
		}
	}
	return nil
}

func fileRangesOverlap(left, right macho.SegmentHeader) bool {
	if left.Filesz == 0 || right.Filesz == 0 {
		return false
	}
	return left.Offset < right.Offset+right.Filesz && right.Offset < left.Offset+left.Filesz
}
