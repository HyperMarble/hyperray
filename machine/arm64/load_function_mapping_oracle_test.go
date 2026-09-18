// This file provides an independent whole-image oracle from debug/macho.
package arm64_test

import (
	"bytes"
	"debug/macho"
	"sort"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func assertFixtureMapping(t *testing.T, file *macho.File, content []byte, image machine.Image) {
	t.Helper()
	// Only the pages a proof can reach are loaded, so every loaded byte must
	// match the file, and no byte may be loaded that the file does not state.
	expected := expectedFixtureBytes(t, file, content)
	byAddress := make(map[uint64]machine.LoadedByte, len(expected))
	for _, want := range expected {
		byAddress[want.Address] = want
	}
	if len(image.LoadedBytes) == 0 {
		t.Fatal("no bytes were loaded")
	}
	for index, actual := range image.LoadedBytes {
		if index > 0 && image.LoadedBytes[index-1].Address >= actual.Address {
			t.Fatalf("loaded addresses are not monotonic and unique at index %d", index)
		}
		want, stated := byAddress[actual.Address]
		if !stated {
			t.Fatalf("loaded byte at 0x%x is not stated by the file", actual.Address)
		}
		if actual != want {
			t.Fatalf("loaded byte[%d] = %+v, want %+v", index, actual, want)
		}
	}
}

func expectedFixtureBytes(t *testing.T, file *macho.File, content []byte) []machine.LoadedByte {
	t.Helper()
	expected := make([]machine.LoadedByte, 0)
	for _, segment := range fixtureSegments(file) {
		if segment.Name == "__PAGEZERO" {
			continue
		}
		permissions := machine.Permissions{Readable: segment.Prot&1 != 0, Writable: segment.Prot&2 != 0, Executable: segment.Prot&4 != 0}
		for offset := uint64(0); offset < segment.Memsz; offset++ {
			value := byte(0)
			if offset < segment.Filesz {
				position := segment.Offset + offset
				if position >= uint64(len(content)) {
					t.Fatalf("fixture segment %q file offset %#x is outside content", segment.Name, position)
				}
				value = content[position]
			}
			expected = append(expected, machine.LoadedByte{Address: segment.Addr + offset, Value: value, Permissions: permissions})
		}
	}
	sort.Slice(expected, func(left, right int) bool { return expected[left].Address < expected[right].Address })
	return expected
}

func assertFixtureInstructions(t *testing.T, text *macho.Section, content []byte, image machine.Image) {
	t.Helper()
	want := []machine.Instruction{
		{Address: text.Addr, Bytes: append([]byte(nil), content[text.Offset:text.Offset+4]...)},
		{Address: text.Addr + 4, Bytes: append([]byte(nil), content[text.Offset+4:text.Offset+8]...)},
	}
	if len(image.Instructions) != len(want) {
		t.Fatalf("instructions = %d, want %d", len(image.Instructions), len(want))
	}
	for index, actual := range image.Instructions {
		if actual.Address != want[index].Address || !bytes.Equal(actual.Bytes, want[index].Bytes) {
			t.Fatalf("instruction[%d] = %+v, want %+v", index, actual, want[index])
		}
	}
}
