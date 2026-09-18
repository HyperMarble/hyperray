// Catalog reader tests use temporary data with no architecture-specific names.
// They never depend on a generated Sail artifact.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeCatalog(t *testing.T, value catalog) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func TestReadCatalogPreservesData(t *testing.T) {
	want := validCatalog()
	got, err := readCatalog(writeCatalog(t, want))
	if err != nil {
		t.Fatalf("readCatalog() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readCatalog() = %#v, want %#v", got, want)
	}
}

func TestReadCatalogRejectsBadInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if _, err := readCatalog(path); err == nil {
		t.Error("readCatalog() error = nil")
	}
}
