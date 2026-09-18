// These tests cover ownership, names, reserved words, and input ownership.
// They must not infer section type, mapping, or execution support.
package arm64_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadSectionHeadersPreservesFieldsAndOwners(t *testing.T) {
	first := sectionFixture{name: "0123456789abcdef", segment: "SEGMENT-ONE-123", addr: 0x1000, size: 0x20, offset: 0x80, align: 3, reloff: 0x90, nreloc: 2, flags: 0x80000400, reserved1: 0x11, reserved2: 0x22, reserved3: 0x33}
	second := sectionFixture{name: "prefix\x00suffix", segment: "SEGMENT-ONE-123", addr: 0x2000, size: 0x30, offset: 0xa0, align: 4, reloff: 0xb0, nreloc: 3, flags: 0x44, reserved1: 0x55, reserved2: 0x66, reserved3: 0x77}
	third := sectionFixture{name: "third", segment: "SEGMENT-TWO-12", addr: 0x3000, size: 0x40, offset: 0xc0, align: 5, reloff: 0xd0, nreloc: 4, flags: 0x88, reserved1: 0x99, reserved2: 0xaa, reserved3: 0xbb}
	firstCommand := sectionCommand("SEGMENT-ONE-123", first, second)
	secondCommand := sectionCommand("SEGMENT-TWO-12", third)
	table := append(syntheticCommand(0x12345678, 8), firstCommand...)
	table = append(table, syntheticCommand(0x87654321, 8)...)
	table = append(table, secondCommand...)
	content := syntheticFile(4, table)
	got, err := arm64.ReadSectionHeaders(content)
	if err != nil {
		t.Fatalf("ReadSectionHeaders() error = %v", err)
	}
	want := []arm64.SectionRecord{
		sectionRecord(1, 0, firstCommand, first),
		sectionRecord(1, 1, firstCommand, second),
		sectionRecord(3, 0, secondCommand, third),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadSectionHeaders() = %#v, want %#v", got, want)
	}
}

func TestReadSectionHeadersOwnsNamesAfterInputMutation(t *testing.T) {
	section := sectionFixture{name: "owned", segment: "OWNED-SEG"}
	content := syntheticFile(1, sectionCommand("OWNED-SEG", section))
	got, err := arm64.ReadSectionHeaders(content)
	if err != nil {
		t.Fatalf("ReadSectionHeaders() error = %v", err)
	}
	want := append([]arm64.SectionRecord(nil), got...)
	for index := range content {
		content[index] = 0
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("records changed after input mutation: got %#v, want %#v", got, want)
	}
}
