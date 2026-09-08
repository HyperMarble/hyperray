// This file validates the source-defined shape of one 64-bit segment command.
// It must not interpret sections or apply segment runtime policy.
package arm64

import (
	"encoding/binary"
	"fmt"
)

const (
	// The SDK and debug/macho define segment_command_64 and section_64 as 72 and 80 bytes.
	segmentCommand64Size = 72
	section64Size        = 80
)

func validateSegmentCommandShape(command []byte, commandIndex uint32, fileOffset uint64, order binary.ByteOrder) error {
	commandSize := uint64(order.Uint32(command[4:8]))
	if commandSize < segmentCommand64Size {
		return &Rejection{Code: InvalidSegmentCommandShape, Detail: fmt.Sprintf("command[%d] fileoffset=%d: LC_SEGMENT_64 cmdsize=%d is less than 72", commandIndex, fileOffset, commandSize)}
	}
	sectionBytes := commandSize - segmentCommand64Size
	nsect := uint64(order.Uint32(command[64:68]))
	recordCount := sectionBytes / section64Size
	remainder := sectionBytes % section64Size
	if remainder != 0 || recordCount != nsect {
		return &Rejection{Code: InvalidSegmentCommandShape, Detail: fmt.Sprintf("command[%d] fileoffset=%d: LC_SEGMENT_64 cmdsize=%d section_bytes=%d records=%d remainder=%d nsect=%d", commandIndex, fileOffset, commandSize, sectionBytes, recordCount, remainder, nsect)}
	}
	return nil
}
