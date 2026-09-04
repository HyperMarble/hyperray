// Artifact tests mutate a temporary input after its digest is recorded.
// They require stale content to remain an engine error.
package isla_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicArtifactIdentity(t *testing.T) {
	path, digest := artifactFixture(t, "model")
	artifact, err := isla.NewArtifact(path, digest)
	if err != nil {
		t.Fatalf("NewArtifact() error = %v", err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if artifact.Path() != absolute || artifact.Digest() != digest {
		t.Errorf("artifact = %q %q", artifact.Path(), artifact.Digest())
	}
}

func TestArtifactRejectsWrongDigest(t *testing.T) {
	path, sourceDigest := artifactFixture(t, "model")
	artifact, err := isla.NewArtifact(path, strings.Repeat("0", 64))
	var failure *isla.Error
	if !errors.As(err, &failure) || failure.Code != isla.ArtifactChanged {
		t.Errorf("NewArtifact() = %#v, source digest %q, error %v", artifact, sourceDigest, err)
	}
}

func artifactFixture(t *testing.T, content string) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	digest := sha256.Sum256([]byte(content))
	return path, hex.EncodeToString(digest[:])
}
