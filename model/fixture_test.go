// These tests apply the public model API to every canonical JSON fixture.
// They never select behavior from a fixture file name.
package model_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestCanonicalModelFixtures(t *testing.T) {
	paths, err := filepath.Glob("fixtures/*.json")
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("fixtures/*.json matched no files")
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", path, err)
		}
		var graph model.Model
		if err := json.Unmarshal(content, &graph); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
		}
		if err := model.Validate(graph); err != nil {
			t.Errorf("Validate(%q) error = %v", path, err)
		}
		encoded, err := json.Marshal(graph)
		if err != nil {
			t.Fatalf("json.Marshal(%q) error = %v", path, err)
		}
		if !bytes.Equal(encoded, bytes.TrimSpace(content)) {
			t.Errorf("fixture %q re-encoded as %s", path, encoded)
		}
	}
}
