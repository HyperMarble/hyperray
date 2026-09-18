// This external helper reads the checked-in ARM64 Mach-O fixture.
// It must not execute, regenerate, or modify the fixture on disk.
package arm64_test

import (
	"debug/macho"
	"os"
	"path/filepath"
	"testing"
)

func fixtureContent(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "machine", "arm64", "tiny-arm64-static"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	return content
}

func openFixture(t *testing.T) *macho.File {
	t.Helper()
	file, err := macho.Open(filepath.Join("..", "..", "fixtures", "machine", "arm64", "tiny-arm64-static"))
	if err != nil {
		t.Fatalf("macho.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("macho.File.Close() error = %v", err)
		}
	})
	return file
}

func arm64Text(t *testing.T, file *macho.File) *macho.Section {
	t.Helper()
	for _, section := range file.Sections {
		if section.Seg == "__TEXT" && section.Name == "__text" {
			return section
		}
	}
	t.Fatal("fixture has no __TEXT,__text section")
	return nil
}
