// These tests cover exact segment file and virtual address boundaries.
// They must not test loading, permissions, or cumulative capacity.
package arm64_test

import (
	"debug/macho"
	"math"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateSegmentRangeAcceptsExactBoundaries(t *testing.T) {
	cases := []struct {
		name         string
		header       macho.SegmentHeader
		artifactSize uint64
	}{
		{name: "address-endpoint", header: macho.SegmentHeader{Name: "__TEXT", Addr: math.MaxUint64}, artifactSize: 0},
		{name: "address-endpoint-with-one-byte-memory", header: macho.SegmentHeader{Name: "__TEXT_END", Addr: math.MaxUint64 - 1, Memsz: 1}, artifactSize: 0},
		{name: "file-endpoint", header: macho.SegmentHeader{Name: "__DATA", Offset: 8}, artifactSize: 8},
		{name: "file-endpoint-with-one-byte-file", header: macho.SegmentHeader{Name: "__DATA_END", Offset: 99, Filesz: 1, Memsz: 1}, artifactSize: 100},
		{name: "pagezero-huge-memory", header: macho.SegmentHeader{Name: "__PAGEZERO", Memsz: math.MaxUint64, Offset: 16}, artifactSize: 16},
		{name: "huge-file-endpoint", header: macho.SegmentHeader{Name: "__HUGE", Offset: math.MaxUint64}, artifactSize: math.MaxUint64},
		{name: "zero-bytes-zero-artifact", header: macho.SegmentHeader{Name: "__ZERO", Addr: math.MaxUint64}, artifactSize: 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := arm64.ValidateSegmentRange(testCase.header, testCase.artifactSize); err != nil {
				t.Fatalf("ValidateSegmentRange() error = %v", err)
			}
		})
	}
}

func TestValidateSegmentRangeRejectsOverflowingAddress(t *testing.T) {
	header := macho.SegmentHeader{Name: "__OVERFLOW", Addr: math.MaxUint64, Memsz: 1}
	rejection := requireSegmentRejection(t, header, 0)
	if rejection.Code != arm64.SegmentAddressOverflow {
		t.Fatalf("Code = %q, want %q", rejection.Code, arm64.SegmentAddressOverflow)
	}
	want := `segment "__OVERFLOW": addr=18446744073709551615 memsz=1 overflows uint64`
	if rejection.Detail != want {
		t.Fatalf("Detail = %q, want %q", rejection.Detail, want)
	}
}
