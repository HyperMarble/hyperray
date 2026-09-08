// These tests validate every parsed fixture segment range.
// They must not infer loader or runtime support.
package arm64_test

import (
	"debug/macho"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateSegmentRangeAcceptsParsedFixtureSegments(t *testing.T) {
	path := filepath.Join("..", "..", "fixtures", "machine", "arm64", "tiny-arm64-static")
	artifact, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
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

	visitedSegments := 0
	fileBackedSegments := 0
	for _, load := range file.Loads {
		segment, ok := load.(*macho.Segment)
		if !ok {
			continue
		}
		visitedSegments++
		if segment.Filesz > 0 {
			fileBackedSegments++
		}
		if err := arm64.ValidateSegmentRange(segment.SegmentHeader, uint64(artifact.Size())); err != nil {
			t.Errorf("ValidateSegmentRange(%q) error = %v", segment.Name, err)
		}
	}
	if visitedSegments == 0 {
		t.Fatal("fixture contained no Mach-O segments")
	}
	if fileBackedSegments == 0 {
		t.Fatal("fixture contained no file-backed Mach-O segments")
	}
}
