// A section that states what its bytes mean still supplies bytes. Apple's
// loader.h calls each of these "section with only" a kind of content, and a
// std binary contains several of them.
package arm64_test

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestContentSectionTypesAreLoaded(t *testing.T) {
	loadable := map[string]uint32{
		"S_CSTRING_LITERALS":           0x02,
		"S_LITERAL_POINTERS":           0x05,
		"S_NON_LAZY_SYMBOL_POINTERS":   0x06,
		"S_LAZY_SYMBOL_POINTERS":       0x07,
		"S_SYMBOL_STUBS":               0x08,
		"S_COALESCED":                  0x0b,
		"S_THREAD_LOCAL_REGULAR":       0x11,
		"S_THREAD_LOCAL_VARIABLES":     0x13,
		"S_THREAD_LOCAL_INIT_POINTERS": 0x15,
	}
	for name, kind := range loadable {
		content := fixtureContent(t)
		binary.LittleEndian.PutUint32(content[176+64:176+68], kind)
		_, err := arm64.LoadFunction(content, 32768,
			arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
		var rejection *arm64.Rejection
		if errors.As(err, &rejection) && rejection.Code == arm64.UnsupportedSectionType {
			t.Errorf("%s (0x%02x) was rejected as an unsupported type", name, kind)
		}
	}
}

func TestAnInterposingSectionIsStillRejected(t *testing.T) {
	content := fixtureContent(t)
	binary.LittleEndian.PutUint32(content[176+64:176+68], 0x0d)
	image, err := arm64.LoadFunction(content, 32768,
		arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	assertLoadFailure(t, image, err, arm64.UnsupportedSectionType)
}
