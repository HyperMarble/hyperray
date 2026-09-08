// These tests cover the existing segment range policy after shape validation.
// They must not add permissions, overlap, capacity, or runtime policy here.
package arm64_test

import (
	"debug/macho"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadSegmentHeadersReturnsExistingRangeErrors(t *testing.T) {
	tests := []struct {
		name    string
		segment string
		addr    uint64
		memsz   uint64
		offset  uint64
		filesz  uint64
		want    arm64.RejectionCode
		detail  string
	}{
		{name: "file exceeds memory", segment: "file-segment", memsz: 1, filesz: 2, want: arm64.SegmentFileSizeExceedsMemorySize, detail: "segment \"file-segment\": filesz=2 exceeds memsz=1"},
		{name: "address overflows", segment: "address-segment", addr: ^uint64(0), memsz: 1, want: arm64.SegmentAddressOverflow, detail: "segment \"address-segment\": addr=18446744073709551615 memsz=1 overflows uint64"},
		{name: "file is outside artifact", segment: "outside-segment", memsz: 1, offset: 999, filesz: 1, want: arm64.SegmentOutsideArtifact, detail: "segment \"outside-segment\": offset=999 filesz=1 exceeds artifact size=104"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := syntheticSegment(macho.LoadCmdSegment64, 72, test.segment, 0)
			setSegmentRange(command, test.addr, test.memsz, test.offset, test.filesz)
			assertReaderRejection(t, syntheticFile(1, command), test.want, test.detail)
		})
	}
}
