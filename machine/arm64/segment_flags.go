// This file validates segment flags for the initial ARM64 mapping profile.
// It must not change headers or infer loader, permission, or runtime behavior.
package arm64

import (
	"debug/macho"
	"fmt"
)

// Apple SDK loader.h:420 defines SG_NORELOC as 0x4.
const segmentFlagNoReloc uint32 = 0x4

// Apple SDK loader.h:428 defines SG_READ_ONLY as 0x10, set on __DATA_CONST
// by every current linker. It states what the segment becomes after fixups,
// which does not change the bytes the file supplies.
const segmentFlagReadOnly uint32 = 0x10

const segmentFlagsMapped = segmentFlagNoReloc | segmentFlagReadOnly

// ValidateSegmentFlags accepts only the flags supported by initial mapping.
func ValidateSegmentFlags(header macho.SegmentHeader) error {
	if header.Flag&^segmentFlagsMapped != 0 {
		return &Rejection{
			Code:   UnsupportedSegmentFlags,
			Detail: fmt.Sprintf("segment %q: flags=0x%08x", header.Name, header.Flag),
		}
	}
	return nil
}
