// These external tests cover the initial segment flag mapping policy.
// They must construct literal headers and inspect typed rejections.
package arm64_test

import (
	"debug/macho"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateSegmentFlagsAcceptsOnlyZeroAndNoReloc(t *testing.T) {
	accepted := map[uint32]bool{0: true, 4: true}
	for flags := uint32(0); flags < 32; flags++ {
		header := macho.SegmentHeader{Name: "__TEXT", Flag: flags}
		err := arm64.ValidateSegmentFlags(header)
		if accepted[flags] {
			if err != nil {
				t.Errorf("flags=0x%08x error = %v, want nil", flags, err)
			}
		} else {
			assertUnsupportedFlags(t, header, err)
		}
	}
}

func TestValidateSegmentFlagsRejectsUnknownAndCombinedFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags uint32
	}{
		{name: "high-bit", flags: 0x80000000},
		{name: "maximum", flags: math.MaxUint32},
		{name: "combined-no-reloc", flags: 0x00000005},
	}
	for _, test := range tests {
		header := macho.SegmentHeader{Name: test.name, Flag: test.flags}
		assertUnsupportedFlags(t, header, arm64.ValidateSegmentFlags(header))
	}
}

func TestValidateSegmentFlagsPreservesHeader(t *testing.T) {
	header := macho.SegmentHeader{Name: "__DATA", Flag: 0x14, Addr: 0x1000, Memsz: 0x2000}
	before := header
	assertUnsupportedFlags(t, header, arm64.ValidateSegmentFlags(header))
	if header != before {
		t.Errorf("header = %#v, want unchanged %#v", header, before)
	}
}

func assertUnsupportedFlags(t *testing.T, header macho.SegmentHeader, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("ValidateSegmentFlags(%q, 0x%08x) error = nil", header.Name, header.Flag)
	}
	var rejection *arm64.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("error type = %T, want *arm64.Rejection", err)
	}
	wantDetail := fmt.Sprintf(`segment %q: flags=0x%08x`, header.Name, header.Flag)
	if rejection.Code != arm64.UnsupportedSegmentFlags || rejection.Detail != wantDetail {
		t.Errorf("rejection = %#v, want code %q and detail %q", rejection, arm64.UnsupportedSegmentFlags, wantDetail)
	}
}
