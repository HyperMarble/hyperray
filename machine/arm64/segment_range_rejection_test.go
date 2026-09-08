// These tests cover the segment range policy and its precedence.
// They must assert exact public codes and deterministic details.
package arm64_test

import (
	"debug/macho"
	"math"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateSegmentRangeRejectsEachInvalidCondition(t *testing.T) {
	cases := []struct {
		name         string
		header       macho.SegmentHeader
		artifactSize uint64
		code         arm64.RejectionCode
		detail       string
	}{
		{name: "file-size", header: macho.SegmentHeader{Name: "__FILESZ", Filesz: 2, Memsz: 1, Offset: 101}, artifactSize: 100, code: arm64.SegmentFileSizeExceedsMemorySize, detail: `segment "__FILESZ": filesz=2 exceeds memsz=1`},
		{name: "address-overflow", header: macho.SegmentHeader{Name: "__ADDR", Addr: math.MaxUint64, Memsz: 1, Offset: 101}, artifactSize: 100, code: arm64.SegmentAddressOverflow, detail: `segment "__ADDR": addr=18446744073709551615 memsz=1 overflows uint64`},
		{name: "past-endpoint", header: macho.SegmentHeader{Name: "__OFFSET", Offset: 101}, artifactSize: 100, code: arm64.SegmentOutsideArtifact, detail: `segment "__OFFSET": offset=101 filesz=0 exceeds artifact size=100`},
		{name: "truncated", header: macho.SegmentHeader{Name: "__TRUNCATED", Offset: 99, Filesz: 2, Memsz: 2}, artifactSize: 100, code: arm64.SegmentOutsideArtifact, detail: `segment "__TRUNCATED": offset=99 filesz=2 exceeds artifact size=100`},
		{name: "huge-truncated", header: macho.SegmentHeader{Name: "__HUGE", Offset: math.MaxUint64, Filesz: 1, Memsz: 1}, artifactSize: math.MaxUint64, code: arm64.SegmentOutsideArtifact, detail: `segment "__HUGE": offset=18446744073709551615 filesz=1 exceeds artifact size=18446744073709551615`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rejection := requireSegmentRejection(t, testCase.header, testCase.artifactSize)
			if rejection.Code != testCase.code || rejection.Detail != testCase.detail {
				t.Fatalf("rejection = %q: %q, want %q: %q", rejection.Code, rejection.Detail, testCase.code, testCase.detail)
			}
		})
	}
}

func TestValidateSegmentRangeUsesPolicyOrder(t *testing.T) {
	header := macho.SegmentHeader{Name: "__ORDER", Filesz: 2, Memsz: 1, Addr: math.MaxUint64, Offset: 101}
	rejection := requireSegmentRejection(t, header, 100)
	if rejection.Code != arm64.SegmentFileSizeExceedsMemorySize {
		t.Fatalf("Code = %q, want %q", rejection.Code, arm64.SegmentFileSizeExceedsMemorySize)
	}
	want := `segment "__ORDER": filesz=2 exceeds memsz=1`
	if rejection.Detail != want {
		t.Fatalf("Detail = %q, want %q", rejection.Detail, want)
	}
}
