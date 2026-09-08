// This file validates individual Mach-O segment file and VM ranges.
// It must not parse, allocate, check permissions, overlap, capacity, or runtime support.
package arm64

import (
	"debug/macho"
	"fmt"
	"math"
)

// ValidateSegmentRange checks one segment against its VM and artifact bounds.
func ValidateSegmentRange(header macho.SegmentHeader, artifactSize uint64) error {
	if header.Filesz > header.Memsz {
		return &Rejection{Code: SegmentFileSizeExceedsMemorySize, Detail: fmt.Sprintf("segment %q: filesz=%d exceeds memsz=%d", header.Name, header.Filesz, header.Memsz)}
	}
	if header.Memsz > math.MaxUint64-header.Addr {
		return &Rejection{Code: SegmentAddressOverflow, Detail: fmt.Sprintf("segment %q: addr=%d memsz=%d overflows uint64", header.Name, header.Addr, header.Memsz)}
	}
	if header.Offset > artifactSize || header.Filesz > artifactSize-header.Offset {
		return &Rejection{Code: SegmentOutsideArtifact, Detail: fmt.Sprintf("segment %q: offset=%d filesz=%d exceeds artifact size=%d", header.Name, header.Offset, header.Filesz, artifactSize)}
	}
	return nil
}
