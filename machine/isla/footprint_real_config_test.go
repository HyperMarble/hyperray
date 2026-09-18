//go:build isla_integration

// Real footprint configuration creates identified valid and invalid artifacts.
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
	return realArtifact(t, "HYPERRAY_ISLA_CONFIG")
}

func realUnknownRegisterConfiguration(t *testing.T) isla.Artifact {
	t.Helper()
	source := requiredPath(t, "HYPERRAY_ISLA_CONFIG")
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	section := []byte("[registers.defaults]\n")
	invalid := append(section, []byte("hyperray_unknown_register = \"0\"\n")...)
	changed := bytes.Replace(content, section, invalid, 1)
	if bytes.Equal(content, changed) {
		t.Fatal("real configuration lacks registers.defaults")
	}
	path := filepath.Join(t.TempDir(), "invalid-register.toml")
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return identifiedArtifact(t, path)
}
