// This external test covers conservative command admission and layout checks.
// It mutates copies of the checked-in Mach-O fixture only.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestLoadFunctionRejectsUnknownCommand(t *testing.T) {
	content := fixtureContent(t)
	setCommandID(content, 0x1b, 0xdeadbeef)
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.UnsupportedLoadCommand)
}

func TestLoadFunctionRejectsMalformedThreadState(t *testing.T) {
	content := fixtureContent(t)
	boundary := arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0}
	if _, err := arm64.LoadFunction(content, 32768, boundary); err != nil {
		t.Fatalf("baseline LoadFunction() error = %v", err)
	}
	commandIndex := setCommandField(content, uint32(macho.LoadCmdUnixThread), 7, 8)
	image, err := arm64.LoadFunction(content, 32768, boundary)
	assertLoadFailure(t, image, err, arm64.InvalidThreadState)
	var rejection *arm64.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("error type = %T, want *arm64.Rejection", err)
	}
	wantDetail := fmt.Sprintf("command[%d]: flavor=7 count=68", commandIndex)
	if rejection.Detail != wantDetail {
		t.Fatalf("rejection detail = %q, want %q", rejection.Detail, wantDetail)
	}
}

func TestLoadFunctionRejectsRuntimeCommand(t *testing.T) {
	content := fixtureContent(t)
	setCommandID(content, 0x2a, uint32(macho.LoadCmdDylinker))
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.UnsupportedLoadCommand)
}

func TestLoadFunctionRejectsOverlappingSegments(t *testing.T) {
	content := fixtureContent(t)
	binary.LittleEndian.PutUint64(content[256+24:256+32], 0x100000300)
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.SegmentOverlap)
}

func setCommandID(content []byte, commandID, replacement uint32) {
	setCommandField(content, commandID, replacement, 0)
}

func setCommandField(content []byte, commandID, replacement uint32, field uint64) uint32 {
	order := binary.LittleEndian
	header := order.Uint32(content[16:20])
	cursor := uint64(32)
	for index := uint32(0); index < header; index++ {
		id := order.Uint32(content[cursor : cursor+4])
		if id == commandID {
			order.PutUint32(content[cursor+field:cursor+field+4], replacement)
			return index
		}
		cursor += uint64(order.Uint32(content[cursor+4 : cursor+8]))
	}
	return ^uint32(0)
}
