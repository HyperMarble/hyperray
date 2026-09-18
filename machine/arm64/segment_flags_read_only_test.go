// A segment marked read-only after fixups still supplies its file bytes, so
// it must be accepted. Every current macOS linker sets it on __DATA_CONST.
package arm64_test

import (
	"debug/macho"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadOnlyAfterFixupsIsAccepted(t *testing.T) {
	header := macho.SegmentHeader{Name: "__DATA_CONST", Flag: 0x10}
	if err := arm64.ValidateSegmentFlags(header); err != nil {
		t.Fatalf("ValidateSegmentFlags() error = %v", err)
	}
}

func TestReadOnlyWithNoRelocIsAccepted(t *testing.T) {
	header := macho.SegmentHeader{Name: "__DATA_CONST", Flag: 0x14}
	if err := arm64.ValidateSegmentFlags(header); err != nil {
		t.Fatalf("ValidateSegmentFlags() error = %v", err)
	}
}

func TestAnUnknownFlagIsStillRejected(t *testing.T) {
	header := macho.SegmentHeader{Name: "__WEIRD", Flag: 0x8}
	if err := arm64.ValidateSegmentFlags(header); err == nil {
		t.Fatal("ValidateSegmentFlags() accepted an unknown flag")
	}
}
