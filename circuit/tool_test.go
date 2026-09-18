// Tool tests use the installed proposal engine through the public constructor.
// They never infer a version from a file name or environment convention.
package circuit_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestDifferenceEngineIdentity(t *testing.T) {
	tool := identifiedEngine(t)
	identity := tool.Identity()
	if !filepath.IsAbs(identity.Path) {
		t.Errorf("ToolIdentity.Path = %q, want an absolute path", identity.Path)
	}
	if !strings.HasPrefix(identity.Version, "Z3 version ") {
		t.Errorf("ToolIdentity.Version = %q", identity.Version)
	}
	content, err := os.ReadFile(identity.Path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", identity.Path, err)
	}
	digest := sha256.Sum256(content)
	wantDigest := "sha256:" + hex.EncodeToString(digest[:])
	if identity.Digest != wantDigest {
		t.Errorf("ToolIdentity.Digest = %q, want %q", identity.Digest, wantDigest)
	}
}

func identifiedEngine(t *testing.T) circuit.DifferenceEngine {
	t.Helper()
	tool, err := circuit.NewDifferenceEngine("")
	if err != nil {
		t.Fatalf("NewDifferenceEngine() error = %v", err)
	}
	return tool
}
