// This external test covers capacity and boundary rejection.
// Each failure must return a zero machine image.
package arm64_test

import (
	"encoding/binary"
	"errors"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestLoadFunctionRejectsCapacityBelowMaterializedMemory(t *testing.T) {
	content := fixtureContent(t)
	image, err := arm64.LoadFunction(content, 32767, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.LoadedByteCapacityExceeded)
}

func TestLoadFunctionRejectsBoundaryOutsidePureCode(t *testing.T) {
	content := fixtureContent(t)
	cases := []arm64.FunctionBoundary{
		{StartAddress: 0x1000002e8, EndAddress: 0x1000002ec - 1},
		{StartAddress: 0x1000002e4, EndAddress: 0x1000002f0},
		{StartAddress: 0x1000002e8, EndAddress: 0x1000002f4},
	}
	for _, boundary := range cases {
		image, err := arm64.LoadFunction(content, 32768, boundary)
		assertLoadFailure(t, image, err, arm64.InvalidFunctionBoundary)
	}
}

func TestLoadFunctionRejectsPureCodeOverMachOMetadata(t *testing.T) {
	content := fixtureContent(t)
	binary.LittleEndian.PutUint64(content[176+32:176+40], 0x100000000)
	binary.LittleEndian.PutUint32(content[176+48:176+52], 0)
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x100000000, EndAddress: 0x100000008})
	assertLoadFailure(t, image, err, arm64.InvalidSection)
}

func assertLoadFailure(t *testing.T, image machine.Image, err error, want arm64.RejectionCode) {
	t.Helper()
	if err == nil {
		t.Fatal("LoadFunction() error = nil")
	}
	if !reflect.DeepEqual(image, machine.Image{}) {
		t.Fatalf("LoadFunction() image = %#v, want zero image", image)
	}
	var rejection *arm64.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("error type = %T, want *arm64.Rejection", err)
	}
	if rejection.Code != want {
		t.Errorf("rejection code = %q, want %q", rejection.Code, want)
	}
}
