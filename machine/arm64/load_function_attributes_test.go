// This external test covers unsupported section and symbol attributes.
// It mutates copies of the checked-in Mach-O fixture only.
package arm64_test

import (
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestLoadFunctionRejectsUnsupportedSectionAttributes(t *testing.T) {
	for _, attribute := range []uint32{0x04000000, 0x00000200, 0x00000100} {
		content := fixtureContent(t)
		putUint32(content, 176+64, getUint32(content, 176+64)|attribute)
		image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
		assertLoadFailure(t, image, err, arm64.UnsupportedSectionAttributes)
	}
}

func TestAPreboundUndefinedSymbolDoesNotRejectTheFile(t *testing.T) {
	// A prebound symbol is defined by another library, which says nothing
	// about the bytes in this file.
	content := fixtureContent(t)
	content[16384+16+4] = 0x0c
	_, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	var rejection *arm64.Rejection
	if errors.As(err, &rejection) && rejection.Code == arm64.UndefinedSymbol {
		t.Fatal("LoadFunction() rejected a file for a prebound symbol")
	}
}

func TestLoadFunctionRejectsUnsupportedSymbolType(t *testing.T) {
	content := fixtureContent(t)
	content[16384+16+4] = 0x04
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.UnsupportedSymbolType)
}

func getUint32(content []byte, offset int) uint32 {
	return uint32(content[offset]) | uint32(content[offset+1])<<8 | uint32(content[offset+2])<<16 | uint32(content[offset+3])<<24
}

func putUint32(content []byte, offset int, value uint32) {
	content[offset] = byte(value)
	content[offset+1] = byte(value >> 8)
	content[offset+2] = byte(value >> 16)
	content[offset+3] = byte(value >> 24)
}
