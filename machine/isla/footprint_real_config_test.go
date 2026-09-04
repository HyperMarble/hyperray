//go:build isla_integration

// Real footprint configuration removes one stale upstream register default.
// Production release configurations must contain no unclassified warning.
package isla_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func realFootprintConfiguration(t *testing.T) isla.Artifact {
	t.Helper()
	source := requiredPath(t, "HYPERRAY_ISLA_CONFIG")
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	staleDefault := []byte("__isla_always_aligned = \"true\"\n")
	cleaned := bytes.Replace(content, staleDefault, nil, 1)
	if bytes.Equal(content, cleaned) {
		t.Fatal("real configuration lacks the measured stale default")
	}
	path := filepath.Join(t.TempDir(), "riscv64-footprint.toml")
	if err := os.WriteFile(path, cleaned, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return identifiedArtifact(t, path)
}
