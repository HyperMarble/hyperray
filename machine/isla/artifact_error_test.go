// Artifact error tests cover malformed identities and files that disappear.
// They require public constructors to reject each bad input.
package isla_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestArtifactInputErrors(t *testing.T) {
	path, digest := artifactFixture(t, "artifact")
	cases := []struct {
		path   string
		digest string
	}{
		{path: "relative", digest: digest},
		{path: path, digest: "short"},
		{path: path, digest: strings.Repeat("g", 64)},
		{path: filepath.Join(t.TempDir(), "missing"), digest: digest},
	}
	for index := range cases {
		testCase := cases[index]
		artifact, err := isla.NewArtifact(testCase.path, testCase.digest)
		var failure *isla.Error
		if !errors.As(err, &failure) || failure.Code != isla.InvalidInput {
			t.Errorf("NewArtifact(%q) = %#v, %v", testCase.path, artifact, err)
		}
	}
}

func TestArtifactRemovalIsObservable(t *testing.T) {
	path, digest := artifactFixture(t, "artifact")
	artifact, err := isla.NewArtifact(path, digest)
	if err != nil {
		t.Fatalf("NewArtifact() error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove() error = %v", err)
	}
	request, err := isla.NewRequest(artifact, artifact, artifact, artifact, 1, 1)
	if err == nil {
		t.Errorf("NewRequest() = %#v, nil error", request)
	}
	assertErrorCode(t, err, isla.ArtifactChanged)
}
