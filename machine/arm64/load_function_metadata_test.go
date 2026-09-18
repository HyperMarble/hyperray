// This external test covers malformed section and symbol metadata.
// It mutates copies of the checked-in Mach-O fixture only.
package arm64_test

import (
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestLoadFunctionRejectsMalformedSectionShape(t *testing.T) {
	content := fixtureContent(t)
	binary.LittleEndian.PutUint32(content[104+64:104+68], 2)
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.InvalidSegmentCommandShape)
}

func TestLoadFunctionRejectsUnsupportedSectionType(t *testing.T) {
	content := fixtureContent(t)
	binary.LittleEndian.PutUint32(content[176+64:176+68], 0xd)
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.UnsupportedSectionType)
}

func TestLoadFunctionRejectsSectionRelocations(t *testing.T) {
	content := fixtureContent(t)
	binary.LittleEndian.PutUint32(content[176+60:176+64], 1)
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.InvalidSection)
}

func TestLoadFunctionRejectsUndefinedSymbol(t *testing.T) {
	content := fixtureContent(t)
	content[16384+4] = 0
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.UndefinedSymbol)
}

func TestLoadFunctionRejectsInvalidSymbolSection(t *testing.T) {
	content := fixtureContent(t)
	content[16384+16+5] = 2
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.InvalidSymbolSection)
}
