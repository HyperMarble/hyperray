// This external test covers segment flags from the checked-in Mach-O fixture.
// It must not regenerate the fixture or infer loader and runtime behavior.
package arm64_test

import (
	"debug/macho"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateSegmentFlagsChecksCheckedInFixture(t *testing.T) {
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

	checkedSegments := 0
	for _, load := range file.Loads {
		segment, ok := load.(*macho.Segment)
		if !ok {
			continue
		}
		checkedSegments++
		if err := arm64.ValidateSegmentFlags(segment.SegmentHeader); err != nil {
			t.Errorf("ValidateSegmentFlags(%q, 0x%08x) error = %v", segment.Name, segment.Flag, err)
		}
	}
	if checkedSegments == 0 {
		t.Fatal("fixture contained no Mach-O segment flags")
	}
}
