// This test passes a real Rust-derived Mach-O through debug/macho.
// It must not infer loader, dyld, or runtime support from header success.
package arm64_test

import (
	"debug/macho"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestValidateHeaderAcceptsParsedRustMachO(t *testing.T) {
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

	if err := arm64.ValidateHeader(file.FileHeader, file.ByteOrder); err != nil {
		t.Fatalf("ValidateHeader(%q) error = %v", path, err)
	}
}
