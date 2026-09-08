// These tests cover literal ARM64 protection cases from the SDK contract.
// They must not compute expected permissions from the implementation.
package arm64_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestSegmentPermissionsLiteralMatrixZeroThroughThree(t *testing.T) {
	cases := []segmentPermissionCase{
		{name: "p0-m0", prot: 0, maxprot: 0, want: machine.Permissions{}},
		{name: "p0-m1", prot: 0, maxprot: 1, want: machine.Permissions{}},
		{name: "p0-m2", prot: 0, maxprot: 2, want: machine.Permissions{}},
		{name: "p0-m3", prot: 0, maxprot: 3, want: machine.Permissions{}},
		{name: "p0-m4", prot: 0, maxprot: 4, want: machine.Permissions{}},
		{name: "p0-m5", prot: 0, maxprot: 5, want: machine.Permissions{}},
		{name: "p0-m6", prot: 0, maxprot: 6, want: machine.Permissions{}},
		{name: "p0-m7", prot: 0, maxprot: 7, want: machine.Permissions{}},
		{name: "p1-m0", prot: 1, maxprot: 0, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p1-m0": initprot=1 exceeds maxprot=0`},
		{name: "p1-m1", prot: 1, maxprot: 1, want: machine.Permissions{Readable: true}},
		{name: "p1-m2", prot: 1, maxprot: 2, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p1-m2": initprot=1 exceeds maxprot=2`},
		{name: "p1-m3", prot: 1, maxprot: 3, want: machine.Permissions{Readable: true}},
		{name: "p1-m4", prot: 1, maxprot: 4, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p1-m4": initprot=1 exceeds maxprot=4`},
		{name: "p1-m5", prot: 1, maxprot: 5, want: machine.Permissions{Readable: true}},
		{name: "p1-m6", prot: 1, maxprot: 6, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p1-m6": initprot=1 exceeds maxprot=6`},
		{name: "p1-m7", prot: 1, maxprot: 7, want: machine.Permissions{Readable: true}},
		{name: "p2-m0", prot: 2, maxprot: 0, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p2-m0": initprot=2 exceeds maxprot=0`},
		{name: "p2-m1", prot: 2, maxprot: 1, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p2-m1": initprot=2 exceeds maxprot=1`},
		{name: "p2-m2", prot: 2, maxprot: 2, want: machine.Permissions{Writable: true}},
		{name: "p2-m3", prot: 2, maxprot: 3, want: machine.Permissions{Writable: true}},
		{name: "p2-m4", prot: 2, maxprot: 4, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p2-m4": initprot=2 exceeds maxprot=4`},
		{name: "p2-m5", prot: 2, maxprot: 5, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p2-m5": initprot=2 exceeds maxprot=5`},
		{name: "p2-m6", prot: 2, maxprot: 6, want: machine.Permissions{Writable: true}},
		{name: "p2-m7", prot: 2, maxprot: 7, want: machine.Permissions{Writable: true}},
		{name: "p3-m0", prot: 3, maxprot: 0, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p3-m0": initprot=3 exceeds maxprot=0`},
		{name: "p3-m1", prot: 3, maxprot: 1, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p3-m1": initprot=3 exceeds maxprot=1`},
		{name: "p3-m2", prot: 3, maxprot: 2, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p3-m2": initprot=3 exceeds maxprot=2`},
		{name: "p3-m3", prot: 3, maxprot: 3, want: machine.Permissions{Readable: true, Writable: true}},
		{name: "p3-m4", prot: 3, maxprot: 4, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p3-m4": initprot=3 exceeds maxprot=4`},
		{name: "p3-m5", prot: 3, maxprot: 5, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p3-m5": initprot=3 exceeds maxprot=5`},
		{name: "p3-m6", prot: 3, maxprot: 6, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "p3-m6": initprot=3 exceeds maxprot=6`},
		{name: "p3-m7", prot: 3, maxprot: 7, want: machine.Permissions{Readable: true, Writable: true}},
	}
	runSegmentPermissionCases(t, cases)
}
