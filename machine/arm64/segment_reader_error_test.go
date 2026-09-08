// These tests cover unsupported commands, range order, and all-or-nothing errors.
// They must keep outer table errors and existing range errors observable.
package arm64_test

import (
	"debug/macho"
	"errors"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadSegmentHeadersRejects32BitSegment(t *testing.T) {
	content := syntheticFile(1, syntheticSegment(macho.LoadCmdSegment, 72, "wrong", 0))
	rejection := requireReaderRejection(t, content)
	if rejection.Code != arm64.UnsupportedSegmentCommand {
		t.Errorf("Code = %q, want %q", rejection.Code, arm64.UnsupportedSegmentCommand)
	}
	if rejection.Detail != "command[0] fileoffset=32: LC_SEGMENT is unsupported" {
		t.Errorf("Detail = %q, want command index and file offset", rejection.Detail)
	}
}

func TestReadSegmentHeadersChecksShapeBeforeRange(t *testing.T) {
	command := syntheticSegment(macho.LoadCmdSegment64, 80, "bad-shape", 0)
	setSegmentRange(command, 0, 0, 999, 1)
	assertReaderRejection(t, syntheticFile(1, command), arm64.InvalidSegmentCommandShape, "command[0] fileoffset=32: LC_SEGMENT_64 cmdsize=80 section_bytes=8 records=0 remainder=8 nsect=0")
}

func TestReadSegmentHeadersForwardsOuterTableError(t *testing.T) {
	content := syntheticHeader(1, 0)
	want := arm64.ValidateCommandTable(content)
	headers, got := arm64.ReadSegmentHeaders(content)
	if headers != nil || got == nil {
		t.Fatalf("ReadSegmentHeaders() = %#v, %v; want nil headers and error", headers, got)
	}
	var wantRejection, gotRejection *arm64.Rejection
	if !errors.As(want, &wantRejection) || !errors.As(got, &gotRejection) {
		t.Fatalf("error types = %T and %T, want *arm64.Rejection", want, got)
	}
	if !reflect.DeepEqual(gotRejection, wantRejection) {
		t.Errorf("rejection = %#v, want %#v", gotRejection, wantRejection)
	}
}

func TestReadSegmentHeadersReturnsNilAfterEarlierValidSegment(t *testing.T) {
	first := syntheticSegment(macho.LoadCmdSegment64, 72, "valid", 0)
	second := syntheticSegment(macho.LoadCmdSegment64, 80, "invalid", 0)
	content := syntheticFile(2, append(first, second...))
	rejection := requireReaderRejection(t, content)
	wantDetail := "command[1] fileoffset=104: LC_SEGMENT_64 cmdsize=80 section_bytes=8 records=0 remainder=8 nsect=0"
	if rejection.Code != arm64.InvalidSegmentCommandShape || rejection.Detail != wantDetail {
		t.Errorf("rejection = %#v, want second-command shape rejection", rejection)
	}
}
