// Identity tests use a real child process and an independently measured digest.
// They never infer identity from an executable name.
package isla_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicEngineIdentity(t *testing.T) {
	path := fixtureTool(t)
	engine, err := isla.NewEngine(context.Background(), path)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	wantDigest := sha256.Sum256(content)
	want := hex.EncodeToString(wantDigest[:])
	identity := engine.Identity()
	if identity.Path != path || identity.Version != "v0.2.0/test" {
		t.Errorf("Identity() = %#v", identity)
	}
	if identity.Digest != want {
		t.Errorf("Identity().Digest = %q, want %q", identity.Digest, want)
	}
}

func fixtureTool(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../fixtures/isla/fake-isla.sh")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("os.Chmod() error = %v", err)
	}
	return path
}
