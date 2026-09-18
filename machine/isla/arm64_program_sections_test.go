// ARM64 section tests compare generated data against the public loader image.
// They cover exact addresses, zero-fill bytes, and exposed writable permissions.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramPreservesLoadedSectionsExactly(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: start, EndAddress: end})
	if err != nil {
		t.Fatalf("LoadFunction() error = %v", err)
	}
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	sections, err := parseARM64Sections(string(program.Content()))
	if err != nil {
		t.Fatalf("parseARM64Sections() error = %v", err)
	}
	assertARM64Sections(t, sections, expectedARM64Sections(image.LoadedBytes))
}

type expectedARM64Section struct {
	address     uint64
	permissions machine.Permissions
	bytes       []byte
}

func expectedARM64Sections(loaded []machine.LoadedByte) []expectedARM64Section {
	sections := make([]expectedARM64Section, 0)
	for _, value := range loaded {
		if len(sections) == 0 || !adjacentARM64Byte(sections[len(sections)-1], value) {
			sections = append(sections, expectedARM64Section{address: value.Address, permissions: value.Permissions})
		}
		sections[len(sections)-1].bytes = append(sections[len(sections)-1].bytes, value.Value)
	}
	return sections
}

func adjacentARM64Byte(section expectedARM64Section, value machine.LoadedByte) bool {
	return section.address+uint64(len(section.bytes)) == value.Address && section.permissions == value.Permissions
}

func assertARM64Sections(t *testing.T, actual []renderedARM64Section, expected []expectedARM64Section) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("rendered section count = %d, want %d", len(actual), len(expected))
	}
	for index := range expected {
		if actual[index].address != expected[index].address || actual[index].writable != expected[index].permissions.Writable {
			t.Fatalf("section %d metadata = %#v, want %#v", index, actual[index], expected[index])
		}
		if string(actual[index].bytes) != string(expected[index].bytes) {
			t.Fatalf("section %d bytes differ: got %d, want %d", index, len(actual[index].bytes), len(expected[index].bytes))
		}
	}
}
