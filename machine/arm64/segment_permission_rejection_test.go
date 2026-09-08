// These tests define ARM64 protection rejection precedence.
// They must report the segment name and numeric protection values.
package arm64_test

import (
	"debug/macho"
	"math"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestSegmentPermissionsRejectsUnknownProtectionBits(t *testing.T) {
	cases := []struct {
		name       string
		prot       uint32
		maxprot    uint32
		wantDetail string
	}{
		{name: "unknown-initprot", prot: 8, maxprot: 7, wantDetail: `segment "unknown-initprot": initprot=8 has bits outside 7`},
		{name: "high-initprot", prot: 0x80000000, maxprot: 7, wantDetail: `segment "high-initprot": initprot=2147483648 has bits outside 7`},
		{name: "maximum-initprot", prot: math.MaxUint32, maxprot: 7, wantDetail: `segment "maximum-initprot": initprot=4294967295 has bits outside 7`},
		{name: "unknown-maxprot", prot: 0, maxprot: 8, wantDetail: `segment "unknown-maxprot": maxprot=8 has bits outside 7`},
		{name: "high-maxprot", prot: 0, maxprot: 0x80000000, wantDetail: `segment "high-maxprot": maxprot=2147483648 has bits outside 7`},
		{name: "maximum-maxprot", prot: 7, maxprot: math.MaxUint32, wantDetail: `segment "maximum-maxprot": maxprot=4294967295 has bits outside 7`},
		{name: "both-maximum", prot: math.MaxUint32, maxprot: math.MaxUint32, wantDetail: `segment "both-maximum": initprot=4294967295 has bits outside 7`},
	}
	for _, testCase := range cases {
		header := macho.SegmentHeader{Name: testCase.name, Prot: testCase.prot, Maxprot: testCase.maxprot}
		permissions, err := arm64.SegmentPermissions(header)
		requirePermissionRejection(t, header.Name, permissions, err, arm64.InvalidSegmentProtection, testCase.wantDetail)
	}
}

func TestSegmentPermissionsUsesDeclaredPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		prot       uint32
		maxprot    uint32
		wantCode   arm64.RejectionCode
		wantDetail string
	}{
		{name: "invalid-init-before-max", prot: 8, maxprot: 8, wantCode: arm64.InvalidSegmentProtection, wantDetail: `segment "invalid-init-before-max": initprot=8 has bits outside 7`},
		{name: "invalid-init-before-wx", prot: 14, maxprot: 7, wantCode: arm64.InvalidSegmentProtection, wantDetail: `segment "invalid-init-before-wx": initprot=14 has bits outside 7`},
		{name: "invalid-max-before-exceeds-wx", prot: 6, maxprot: 8, wantCode: arm64.InvalidSegmentProtection, wantDetail: `segment "invalid-max-before-exceeds-wx": maxprot=8 has bits outside 7`},
		{name: "exceeds-before-wx", prot: 6, maxprot: 1, wantCode: arm64.SegmentProtectionExceedsMaximum, wantDetail: `segment "exceeds-before-wx": initprot=6 exceeds maxprot=1`},
		{name: "wx-last", prot: 6, maxprot: 7, wantCode: arm64.WritableExecutableSegment, wantDetail: `segment "wx-last": initprot=6 is writable and executable`},
	}
	for _, testCase := range cases {
		header := macho.SegmentHeader{Name: testCase.name, Prot: testCase.prot, Maxprot: testCase.maxprot}
		permissions, err := arm64.SegmentPermissions(header)
		requirePermissionRejection(t, header.Name, permissions, err, testCase.wantCode, testCase.wantDetail)
	}
}

func requirePermissionRejection(t *testing.T, name string, permissions machine.Permissions, err error, wantCode arm64.RejectionCode, wantDetail string) {
	t.Helper()
	if err == nil || permissions != (machine.Permissions{}) {
		t.Fatalf("%s: permissions=%+v err=%v, want zero permissions and %q", name, permissions, err, wantCode)
	}
	rejection, ok := err.(*arm64.Rejection)
	if !ok {
		t.Fatalf("%s: error type=%T, want *arm64.Rejection", name, err)
	}
	if rejection.Code != wantCode || rejection.Detail != wantDetail || err.Error() != string(wantCode)+": "+wantDetail {
		t.Fatalf("%s: rejection=%q %q, want %q %q", name, rejection.Code, rejection.Detail, wantCode, wantDetail)
	}
}
