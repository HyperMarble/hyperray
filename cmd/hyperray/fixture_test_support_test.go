// Command test support writes fixture files for command tests.
// It does not bypass the command it is testing.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeJSONFixture(t *testing.T, name string, value any) string {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return writeFixture(t, name, content)
}

func writeFixture(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}
