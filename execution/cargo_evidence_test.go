//go:build preparation_integration && darwin

// Preserve input project bytes and retain generated Cargo build evidence.
// Neither a modified source tree nor missing build evidence can pass silently.
package execution_test

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func cargoSnapshot(t *testing.T, directory string) map[string][32]byte {
	t.Helper()
	result := make(map[string][32]byte)
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() {
			result[path] = [32]byte{}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[path] = sha256.Sum256(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func cargoBuildEvidence(t *testing.T, directory string) {
	t.Helper()
	for _, name := range []string{"Cargo.toml", "Cargo.lock", "cargo-build.jsonl", "libbinding.a"} {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			t.Fatalf("missing build evidence: %s", name)
		}
	}
}
