// Release tests exercise strict manifests through the public constructor.
package isla_test

import (
	"os"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicFootprintRelease(t *testing.T) {
	engine, architecture, configuration := releaseInputs(t)
	release := footprintRelease(t, engine, architecture, configuration)
	if release == (isla.FootprintRelease{}) {
		t.Error("NewFootprintRelease() returned a zero release")
	}
}

func TestFootprintReleaseRejectsMalformedManifest(t *testing.T) {
	engine, architecture, configuration := releaseInputs(t)
	valid := matchingManifest(engine, architecture, configuration)
	validJSON := manifestJSON(t, valid)
	cases := []string{
		"{",
		"{\"unknown\":true}",
		validJSON + "{}",
		strings.Replace(validJSON, "test-release", "", 1),
		strings.Replace(validJSON, valid.ToolDigest, "bad", 1),
	}
	for index := range cases {
		manifest := testArtifact(t, cases[index])
		release, err := isla.NewFootprintRelease(manifest, engine, architecture, configuration)
		assertReleaseError(t, index, release, err)
	}
}

func TestFootprintReleaseRejectsUnreadableManifest(t *testing.T) {
	engine, architecture, configuration := releaseInputs(t)
	manifest := manifestArtifact(t, matchingManifest(engine, architecture, configuration))
	if err := os.Remove(manifest.Path()); err != nil {
		t.Fatalf("os.Remove() error = %v", err)
	}
	release, err := isla.NewFootprintRelease(manifest, engine, architecture, configuration)
	assertReleaseError(t, 0, release, err)
}

func releaseInputs(t *testing.T) (isla.FootprintEngine, isla.Artifact, isla.Artifact) {
	t.Helper()
	engine := footprintEngine(t)
	architecture := testArtifact(t, "release-architecture")
	configuration := testArtifact(t, "release-configuration")
	return engine, architecture, configuration
}

func manifestJSON(t *testing.T, manifest testReleaseManifest) string {
	t.Helper()
	return string(mustJSON(t, manifest))
}

func assertReleaseError(t *testing.T, index int, release isla.FootprintRelease, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("case %d release = %#v, error = nil", index, release)
	}
	assertErrorCode(t, err, isla.ReleaseMismatch)
}
