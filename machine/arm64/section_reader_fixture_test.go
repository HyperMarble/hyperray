// These tests compare every public section field with debug/macho.
// They must not use the standard parser as production validation.
package arm64_test

import (
	"debug/macho"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadSectionHeadersMatchesCheckedInFixture(t *testing.T) {
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
	want := fixtureSectionRecords(file)
	got, err := arm64.ReadSectionHeaders(content)
	if err != nil {
		t.Fatalf("ReadSectionHeaders() error = %v", err)
	}
	if len(want) == 0 || len(got) != len(want) {
		t.Fatalf("section count = %d, want at least one and exactly %d", len(got), len(want))
	}
	if !hasFileBackedCodeSection(want) {
		t.Fatal("fixture has no file-backed code section")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadSectionHeaders() = %#v, want %#v", got, want)
	}
}

func fixtureSectionRecords(file *macho.File) []arm64.SectionRecord {
	records := make([]arm64.SectionRecord, 0)
	sectionIndex := uint32(0)
	for commandIndex, load := range file.Loads {
		segment, ok := load.(*macho.Segment)
		if !ok {
			continue
		}
		localIndex := uint32(0)
		for _, section := range file.Sections[sectionIndex:] {
			if section.Seg != segment.Name {
				break
			}
			records = append(records, arm64.SectionRecord{
				CommandIndex: uint32(commandIndex), SectionIndex: localIndex,
				Segment: segment.SegmentHeader, Header: section.SectionHeader,
			})
			sectionIndex++
			localIndex++
		}
	}
	return records
}
