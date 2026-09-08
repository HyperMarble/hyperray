// These tests define the public rejection string.
// They must preserve both the named code and the exact detail.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestRejectionErrorIncludesCodeAndDetail(t *testing.T) {
	header := acceptedHeader()
	header.Magic = macho.Magic32
	err := arm64.ValidateHeader(header, binary.LittleEndian)
	if err == nil {
		t.Fatal("ValidateHeader() error = nil")
	}
	if got := err.Error(); got != "unsupported_magic: 0xfeedface" {
		t.Fatalf("Error() = %q, want %q", got, "unsupported_magic: 0xfeedface")
	}
}
