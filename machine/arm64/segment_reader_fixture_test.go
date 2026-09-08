// This test compares metadata with the standard Mach-O parser.
// It must not treat metadata success as loadability or runtime support.
package arm64_test

import (
	"debug/macho"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestReadSegmentHeadersMatchesCheckedInFixture(t *testing.T) {
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

	want := make([]macho.SegmentHeader, 0)
	fileBacked := false
	for _, load := range file.Loads {
		segment, ok := load.(*macho.Segment)
		if !ok {
			continue
		}
		want = append(want, segment.SegmentHeader)
		fileBacked = fileBacked || segment.Filesz > 0
	}
	got, err := arm64.ReadSegmentHeaders(content)
	if err != nil {
		t.Fatalf("ReadSegmentHeaders() error = %v", err)
	}
	if !fileBacked {
		t.Fatal("fixture has no file-backed segment")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadSegmentHeaders() = %#v, want %#v", got, want)
	}
}
