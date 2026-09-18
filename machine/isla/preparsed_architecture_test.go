// A pre-parsed model must be used when one exists, and the text model must be
// kept when it does not, so a missing file never changes the verdict.
package isla

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheTextModelIsKeptWhenNoPreparsedFormExists(t *testing.T) {
	folder := t.TempDir()
	model := filepath.Join(folder, "armv8p5.ir")
	if err := os.WriteFile(model, []byte("text"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if found := PreparsedArchitecture(model); found != model {
		t.Fatalf("PreparsedArchitecture() = %q, want %q", found, model)
	}
}

func TestThePreparsedFormIsUsedWhenItExists(t *testing.T) {
	folder := t.TempDir()
	model := filepath.Join(folder, "armv8p5.ir")
	preparsed := model + ".irx"
	for _, path := range []string{model, preparsed} {
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	if found := PreparsedArchitecture(model); found != preparsed {
		t.Fatalf("PreparsedArchitecture() = %q, want %q", found, preparsed)
	}
}

func TestAnEmptyPreparsedFileIsNotUsed(t *testing.T) {
	folder := t.TempDir()
	model := filepath.Join(folder, "armv8p5.ir")
	for _, path := range []string{model, model + ".irx"} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	if found := PreparsedArchitecture(model); found != model {
		t.Fatalf("PreparsedArchitecture() = %q, want the text model", found)
	}
}

func TestAPreparsedPathIsReturnedUnchanged(t *testing.T) {
	path := "/models/armv8p5.ir.irx"
	if found := PreparsedArchitecture(path); found != path {
		t.Fatalf("PreparsedArchitecture() = %q, want %q", found, path)
	}
}
