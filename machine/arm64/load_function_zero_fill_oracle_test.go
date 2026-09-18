// This file checks accepted zero-fill bytes against the parsed mutated fixture.
package arm64_test

import (
	"bytes"
	"debug/macho"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func assertInsertedZeroFill(t *testing.T, content []byte, image machine.Image, address uint64) {
	t.Helper()
	file := openMachoContent(t, content)
	section := findSection(t, file, address)
	segment := findSegment(t, file, section.Seg)
	assertInsertedCodeIdentity(t, file, content, image)
	permissions := machine.Permissions{Readable: segment.Prot&1 != 0, Writable: segment.Prot&2 != 0, Executable: segment.Prot&4 != 0}
	for offset := uint64(0); offset < section.Size; offset++ {
		loaded := loadedByteAt(t, image, section.Addr+offset)
		if loaded.Value != 0 || loaded.Permissions != permissions {
			t.Fatalf("zero-fill byte at %#x = %+v, want zero with %+v", loaded.Address, loaded, permissions)
		}
	}
	boundary := segment.Addr + segment.Filesz
	previous := loadedByteAt(t, image, boundary-1)
	if previous.Value != content[segment.Offset+segment.Filesz-1] {
		t.Fatalf("file-backed boundary byte at %#x = %#x, want %#x", previous.Address, previous.Value, content[segment.Offset+segment.Filesz-1])
	}
	firstZero := loadedByteAt(t, image, boundary)
	if firstZero.Value != 0 || firstZero.Permissions != permissions {
		t.Fatalf("first zero-fill byte at %#x = %#x, want zero", firstZero.Address, firstZero.Value)
	}
}
func openMachoContent(t *testing.T, content []byte) *macho.File {
	t.Helper()
	file, err := macho.NewFile(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("macho.NewFile() error = %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}
func findSegment(t *testing.T, file *macho.File, name string) *macho.Segment {
	t.Helper()
	for _, segment := range fixtureSegments(file) {
		if segment.Name == name {
			return segment
		}
	}
	t.Fatalf("fixture has no segment %q", name)
	return nil
}

func findSection(t *testing.T, file *macho.File, address uint64) *macho.Section {
	t.Helper()
	for _, section := range file.Sections {
		if section.Addr == address {
			return section
		}
	}
	t.Fatalf("fixture has no section at %#x", address)
	return nil
}

func loadedByteAt(t *testing.T, image machine.Image, address uint64) machine.LoadedByte {
	t.Helper()
	for _, loaded := range image.LoadedBytes {
		if loaded.Address == address {
			return loaded
		}
	}
	t.Fatalf("image has no loaded byte at %#x", address)
	return machine.LoadedByte{}
}
