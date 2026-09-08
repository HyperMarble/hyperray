// These tests cover header decoding before command-table framing.
// Synthetic bytes are named to separate them from the real Rust fixture.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateCommandTableRejectsSyntheticTruncatedHeaders(t *testing.T) {
	assertTableRejection(t, nil, arm64.TruncatedHeader, "need 32 bytes, have 0")
	fullHeader := syntheticHeader(0, 0)
	for length := 0; length < 32; length++ {
		t.Run("length="+strconv.Itoa(length), func(t *testing.T) {
			content := fullHeader[:length]
			assertTableRejection(t, content, arm64.TruncatedHeader, "need 32 bytes, have "+strconv.Itoa(length))
		})
	}
}

func TestValidateCommandTableAcceptsSyntheticEmptyTable(t *testing.T) {
	if err := arm64.ValidateCommandTable(syntheticHeader(0, 0)); err != nil {
		t.Fatalf("ValidateCommandTable() error = %v", err)
	}
}

func TestValidateCommandTableAcceptsRustMachOWithCommands(t *testing.T) {
	path := filepath.Join("..", "..", "fixtures", "machine", "arm64", "tiny-arm64-static")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	file, err := macho.Open(path)
	if err != nil {
		t.Fatalf("macho.Open(%q) error = %v", path, err)
	}
	defer func() {
		if closeError := file.Close(); closeError != nil {
			t.Errorf("macho.File.Close() error = %v", closeError)
		}
	}()
	if file.Ncmd == 0 {
		t.Fatal("Rust Mach-O fixture has no commands")
	}
	if err := arm64.ValidateCommandTable(content); err != nil {
		t.Fatalf("ValidateCommandTable(%q) error = %v", path, err)
	}
}

func TestValidateCommandTableUsesHeaderPolicyOrder(t *testing.T) {
	content := syntheticHeader(0, 0)
	binary.LittleEndian.PutUint32(content[0:4], macho.Magic32)
	binary.LittleEndian.PutUint32(content[4:8], uint32(macho.CpuAmd64))
	assertTableRejection(t, content, arm64.UnsupportedMagic, "0xfeedface")
}

func TestValidateCommandTableIgnoresSyntheticReservedHeaderWord(t *testing.T) {
	content := syntheticHeader(0, 0)
	for index := 28; index < 32; index++ {
		content[index] = 0xff
	}
	if err := arm64.ValidateCommandTable(content); err != nil {
		t.Fatalf("ValidateCommandTable() error = %v", err)
	}
}
