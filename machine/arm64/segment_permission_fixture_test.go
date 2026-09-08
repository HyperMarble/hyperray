// This test maps every parsed fixture segment through the public API.
// It must use the checked-in artifact and must not execute or regenerate it.
package arm64_test

import (
	"debug/macho"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestSegmentPermissionsMapsCheckedInMachOFixture(t *testing.T) {
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
		permissions, err := arm64.SegmentPermissions(segment.SegmentHeader)
		want := machine.Permissions{Readable: segment.Prot&1 != 0, Writable: segment.Prot&2 != 0, Executable: segment.Prot&4 != 0}
		if err != nil || permissions != want {
			t.Errorf("SegmentPermissions(%q) = %+v, %v, want %+v", segment.Name, permissions, err, want)
		}
	}
	if visitedSegments == 0 {
		t.Fatal("fixture contained no Mach-O segments")
	}
	if fileBackedSegments == 0 {
		t.Fatal("fixture contained no file-backed Mach-O segments")
	}
}
