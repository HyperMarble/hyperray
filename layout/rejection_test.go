// These tests make sure that the layout gate rejects mixed language source.
// They must use temporary programs instead of repository fixture names.
package layout

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRejectsSourceInWrongAdapter(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "python", "prove.rs")
	if err := writeSource(path); err != nil {
		t.Fatal(err)
	}
	failures, err := sourceOwnershipFailures(root, sourceOwners)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join("python", "prove.rs")}
	if !reflect.DeepEqual(failures, want) {
		t.Errorf("failures = %v, want %v", failures, want)
	}
}

func TestRejectsUnknownAdapterDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "mixed"), 0o755); err != nil {
		t.Fatal(err)
	}
	failures, err := unsupportedAdapterDirectories(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"mixed"}
	if !reflect.DeepEqual(failures, want) {
		t.Errorf("failures = %v, want %v", failures, want)
	}
}

func writeSource(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("source"), 0o600)
}
