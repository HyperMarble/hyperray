// These tests cover exact LC_SEGMENT_64 command shapes.
// They must distinguish truncation, count mismatch, and extra bytes.
package arm64_test

import (
	"debug/macho"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadSegmentHeadersAcceptsExactSyntheticShapes(t *testing.T) {
	tests := []struct {
		name   string
		size   uint32
		nsect  uint32
		failed bool
		detail string
	}{
		{name: "zero sections", size: 72},
		{name: "one section", size: 152, nsect: 1},
		{name: "count too small", size: 152, failed: true, detail: "command[0] fileoffset=32: LC_SEGMENT_64 cmdsize=152 section_bytes=80 records=1 remainder=0 nsect=0"},
		{name: "count too large", size: 72, nsect: 1, failed: true, detail: "command[0] fileoffset=32: LC_SEGMENT_64 cmdsize=72 section_bytes=0 records=0 remainder=0 nsect=1"},
		{name: "extra bytes", size: 160, nsect: 1, failed: true, detail: "command[0] fileoffset=32: LC_SEGMENT_64 cmdsize=160 section_bytes=88 records=1 remainder=8 nsect=1"},
		{name: "short command", size: 64, failed: true, detail: "command[0] fileoffset=32: LC_SEGMENT_64 cmdsize=64 is less than 72"},
		{name: "huge count", size: 72, nsect: ^uint32(0), failed: true, detail: "command[0] fileoffset=32: LC_SEGMENT_64 cmdsize=72 section_bytes=0 records=0 remainder=0 nsect=4294967295"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := syntheticSegment(macho.LoadCmdSegment64, test.size, "shape", test.nsect)
			content := syntheticFile(1, command)
			if test.failed {
				assertReaderRejection(t, content, arm64.InvalidSegmentCommandShape, test.detail)
				return
			}
			headers, err := arm64.ReadSegmentHeaders(content)
			if err != nil {
				t.Fatalf("ReadSegmentHeaders() error = %v", err)
			}
			if len(headers) != 1 || headers[0].Len != test.size || headers[0].Nsect != test.nsect {
				t.Errorf("headers = %#v, want one header of length %d and Nsect %d", headers, test.size, test.nsect)
			}
		})
	}
}
