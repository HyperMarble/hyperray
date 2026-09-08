// These helpers check literal ARM64 permission cases through the public API.
// They must not derive expected results from the implementation rules.
package arm64_test

import (
	"debug/macho"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

type segmentPermissionCase struct {
	name       string
	prot       uint32
	maxprot    uint32
	want       machine.Permissions
	wantCode   arm64.RejectionCode
	wantDetail string
}

func runSegmentPermissionCases(t *testing.T, cases []segmentPermissionCase) {
	t.Helper()
	for _, testCase := range cases {
		header := macho.SegmentHeader{Name: testCase.name, Prot: testCase.prot, Maxprot: testCase.maxprot}
		permissions, err := arm64.SegmentPermissions(header)
		if testCase.wantCode != "" {
			requirePermissionRejection(t, testCase.name, permissions, err, testCase.wantCode, testCase.wantDetail)
			continue
		}
		if err != nil || permissions != testCase.want {
			t.Errorf("%s: permissions=%+v err=%v, want %+v", testCase.name, permissions, err, testCase.want)
		}
	}
}
