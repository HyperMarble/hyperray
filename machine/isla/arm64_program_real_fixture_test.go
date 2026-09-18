//go:build isla_integration && arm64_acceptance

// This helper proves the checked-in Mach-O function boundary and source identity.
// It must not infer semantics from the raw instruction encodings.
package isla_test

import (
	"bytes"
	"debug/macho"
	"os"
	"path/filepath"
	"testing"
)

var arm64TextBytes = []byte{0x00, 0x04, 0x00, 0x91, 0xc0, 0x03, 0x5f, 0xd6}

func arm64Fixture(t *testing.T) ([]byte, uint64, uint64) {
	t.Helper()
	path := filepath.Join("..", "..", "fixtures", "machine", "arm64", "tiny-arm64-static")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	if len(content) != 16456 {
		t.Fatalf("fixture size = %d, want 16456", len(content))
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
	text := arm64TextSection(t, file)
	if text.Addr != 0x1000002e8 || text.Offset != 744 || text.Size != 8 {
		t.Fatalf("__TEXT,__text = addr %#x offset %d size %d", text.Addr, text.Offset, text.Size)
	}
	end := uint64(text.Offset) + text.Size
	if end > uint64(len(content)) || !bytes.Equal(content[text.Offset:int(end)], arm64TextBytes) {
		t.Fatalf("__TEXT,__text bytes do not match measured fixture bytes")
	}
	arm64TextSymbol(t, file, text.Addr)
	return content, text.Addr, text.Addr + text.Size
}

func assertARM64ContinuationOutsideImage(t *testing.T, address uint64) {
	t.Helper()
	path := filepath.Join("..", "..", "fixtures", "machine", "arm64", "tiny-arm64-static")
	file, err := macho.Open(path)
	if err != nil {
		t.Fatalf("macho.Open(%q) error = %v", path, err)
	}
	defer func() {
		if closeError := file.Close(); closeError != nil {
			t.Errorf("macho.File.Close() error = %v", closeError)
		}
	}()
	segments := 0
	for _, load := range file.Loads {
		segment, ok := load.(*macho.Segment)
		if !ok {
			continue
		}
		segments++
		if address >= segment.Addr && address < segment.Addr+segment.Memsz {
			t.Fatalf("continuation %#x is inside loaded segment %s", address, segment.Name)
		}
	}
	if segments != 3 {
		t.Fatalf("loaded segment count = %d, want 3", segments)
	}
}
