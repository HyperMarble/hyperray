// These tests cover literal ARM64 protection cases from the SDK contract.
// They must not compute expected permissions from the implementation.
package arm64_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestSegmentPermissionsLiteralMatrixFourThroughSeven(t *testing.T) {
	cases := []segmentPermissionCase{
		{name: "p4-m0", prot: 4, maxprot: 0, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p4-m0": initprot=4 exceeds maxprot=0`},
		{name: "p4-m1", prot: 4, maxprot: 1, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p4-m1": initprot=4 exceeds maxprot=1`},
		{name: "p4-m2", prot: 4, maxprot: 2, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p4-m2": initprot=4 exceeds maxprot=2`},
		{name: "p4-m3", prot: 4, maxprot: 3, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p4-m3": initprot=4 exceeds maxprot=3`},
		{name: "p4-m4", prot: 4, maxprot: 4, want: machine.Permissions{Executable: true}},
		{name: "p4-m5", prot: 4, maxprot: 5, want: machine.Permissions{Executable: true}},
		{name: "p4-m6", prot: 4, maxprot: 6, want: machine.Permissions{Executable: true}},
		{name: "p4-m7", prot: 4, maxprot: 7, want: machine.Permissions{Executable: true}},
		{name: "p5-m0", prot: 5, maxprot: 0, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p5-m0": initprot=5 exceeds maxprot=0`},
		{name: "p5-m1", prot: 5, maxprot: 1, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p5-m1": initprot=5 exceeds maxprot=1`},
		{name: "p5-m2", prot: 5, maxprot: 2, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p5-m2": initprot=5 exceeds maxprot=2`},
		{name: "p5-m3", prot: 5, maxprot: 3, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p5-m3": initprot=5 exceeds maxprot=3`},
		{name: "p5-m4", prot: 5, maxprot: 4, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p5-m4": initprot=5 exceeds maxprot=4`},
		{name: "p5-m5", prot: 5, maxprot: 5, want: machine.Permissions{Readable: true, Executable: true}},
		{name: "p5-m6", prot: 5, maxprot: 6, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p5-m6": initprot=5 exceeds maxprot=6`},
		{name: "p5-m7", prot: 5, maxprot: 7, want: machine.Permissions{Readable: true, Executable: true}},
		{name: "p6-m0", prot: 6, maxprot: 0, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p6-m0": initprot=6 exceeds maxprot=0`},
		{name: "p6-m1", prot: 6, maxprot: 1, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p6-m1": initprot=6 exceeds maxprot=1`},
		{name: "p6-m2", prot: 6, maxprot: 2, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p6-m2": initprot=6 exceeds maxprot=2`},
		{name: "p6-m3", prot: 6, maxprot: 3, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p6-m3": initprot=6 exceeds maxprot=3`},
		{name: "p6-m4", prot: 6, maxprot: 4, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p6-m4": initprot=6 exceeds maxprot=4`},
		{name: "p6-m5", prot: 6, maxprot: 5, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p6-m5": initprot=6 exceeds maxprot=5`},
		{name: "p6-m6", prot: 6, maxprot: 6, wantCode: arm64.WritableExecutableSegment, wantDetail: `segment "p6-m6": initprot=6 is writable and executable`},
		{name: "p6-m7", prot: 6, maxprot: 7, wantCode: arm64.WritableExecutableSegment, wantDetail: `segment "p6-m7": initprot=6 is writable and executable`},
		{name: "p7-m0", prot: 7, maxprot: 0, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p7-m0": initprot=7 exceeds maxprot=0`},
		{name: "p7-m1", prot: 7, maxprot: 1, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p7-m1": initprot=7 exceeds maxprot=1`},
		{name: "p7-m2", prot: 7, maxprot: 2, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p7-m2": initprot=7 exceeds maxprot=2`},
		{name: "p7-m3", prot: 7, maxprot: 3, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p7-m3": initprot=7 exceeds maxprot=3`},
		{name: "p7-m4", prot: 7, maxprot: 4, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p7-m4": initprot=7 exceeds maxprot=4`},
		{name: "p7-m5", prot: 7, maxprot: 5, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p7-m5": initprot=7 exceeds maxprot=5`},
		{name: "p7-m6", prot: 7, maxprot: 6, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p7-m6": initprot=7 exceeds maxprot=6`},
		{name: "p7-m7", prot: 7, maxprot: 7, wantCode: arm64.WritableExecutableSegment, wantDetail: `segment "p7-m7": initprot=7 is writable and executable`},
	}
	runSegmentPermissionCases(t, cases)
}
