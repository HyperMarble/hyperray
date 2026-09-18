// This external helper inserts one zero-fill section in a fixture copy.
// It must preserve the Mach-O command and file-offset relationships.
package arm64_test

import (
	"debug/macho"
	"encoding/binary"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func mutatedLinkeditWithZeroFill(t *testing.T, address uint64) []byte {
	return mutatedLinkeditSection(t, address, 1)
}

func mutatedLinkeditWithGreaterZeroFill(t *testing.T, address uint64) []byte {
	return mutatedLinkeditSection(t, address, 12)
}

func mutatedLinkeditSection(t *testing.T, address uint64, sectionType uint32) []byte {
	t.Helper()
	content := fixtureContent(t)
	insertAt := 328
	insertedByteCount := 80
	inserted := make([]byte, len(content)+insertedByteCount)
	copy(inserted, content[:insertAt])
	copy(inserted[insertAt+insertedByteCount:], content[insertAt:])
	binary.LittleEndian.PutUint32(inserted[20:24], 728)
	binary.LittleEndian.PutUint32(inserted[256+4:256+8], 152)
	binary.LittleEndian.PutUint32(inserted[256+64:256+68], 1)
	binary.LittleEndian.PutUint64(inserted[256+40:256+48], 16464)
	text := inserted[176:256]
	binary.LittleEndian.PutUint64(text[32:40], binary.LittleEndian.Uint64(text[32:40])+uint64(insertedByteCount))
	binary.LittleEndian.PutUint32(text[48:52], binary.LittleEndian.Uint32(text[48:52])+uint32(insertedByteCount))
	section := inserted[328:408]
	copy(section[0:16], "__bss")
	copy(section[16:32], "__LINKEDIT")
	binary.LittleEndian.PutUint64(section[32:40], address)
	binary.LittleEndian.PutUint64(section[40:48], 4)
	binary.LittleEndian.PutUint32(section[52:56], 2)
	binary.LittleEndian.PutUint32(section[64:68], sectionType)
	symtab := inserted[408:432]
	binary.LittleEndian.PutUint32(symtab[8:12], 16464)
	binary.LittleEndian.PutUint32(symtab[16:20], 16496)
	binary.LittleEndian.PutUint64(inserted[16464+16+8:16464+16+16], binary.LittleEndian.Uint64(inserted[16464+16+8:16464+16+16])+uint64(insertedByteCount))
	return inserted
}

func assertInsertedCodeIdentity(t *testing.T, file *macho.File, content []byte, image machine.Image) {
	t.Helper()
	originalContent := fixtureContent(t)
	originalFile := openFixture(t)
	originalText := arm64Text(t, originalFile)
	text := arm64Text(t, file)
	insertedByteCount := uint64(len(content) - len(originalContent))
	if text.Addr != originalText.Addr+insertedByteCount || text.Offset != originalText.Offset+uint32(insertedByteCount) {
		t.Fatalf("text identity = addr %#x offset %#x, want addr %#x offset %#x", text.Addr, text.Offset, originalText.Addr+insertedByteCount, originalText.Offset+uint32(insertedByteCount))
	}
	if image.EntryAddress != text.Addr {
		t.Fatalf("entry address = %#x, want text address %#x", image.EntryAddress, text.Addr)
	}
	if file.Symtab == nil {
		t.Fatal("mutated fixture has no symbol table")
	}
	for _, symbol := range file.Symtab.Syms {
		if symbol.Name == "_arm64_fixture" && symbol.Value == text.Addr {
			assertFixtureInstructions(t, text, content, image)
			return
		}
	}
	t.Fatalf("symbol _arm64_fixture does not identify text address %#x", text.Addr)
}
