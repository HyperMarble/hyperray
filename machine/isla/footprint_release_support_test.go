// Release test support writes manifests from independently measured identities.
package isla_test

import (
	"encoding/json"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

type testReleaseManifest struct {
	ReleaseID           string `json:"release_id"`
	ToolVersion         string `json:"tool_version"`
	ToolDigest          string `json:"tool_sha256"`
	ArchitectureDigest  string `json:"architecture_sha256"`
	ConfigurationDigest string `json:"configuration_sha256"`
}

func footprintRelease(t *testing.T, engine isla.FootprintEngine, architecture isla.Artifact, configuration isla.Artifact) isla.FootprintRelease {
	t.Helper()
	manifest := matchingManifest(engine, architecture, configuration)
	manifestArtifact := manifestArtifact(t, manifest)
	release, err := isla.NewFootprintRelease(manifestArtifact, engine, architecture, configuration)
	if err != nil {
		t.Fatalf("NewFootprintRelease() error = %v", err)
	}
	return release
}

func matchingManifest(engine isla.FootprintEngine, architecture isla.Artifact, configuration isla.Artifact) testReleaseManifest {
	identity := engine.Identity()
	return testReleaseManifest{
		ReleaseID: "test-release", ToolVersion: identity.Version,
		ToolDigest: identity.Digest, ArchitectureDigest: architecture.Digest(),
		ConfigurationDigest: configuration.Digest(),
	}
}

func manifestArtifact(t *testing.T, manifest testReleaseManifest) isla.Artifact {
	t.Helper()
	content := mustJSON(t, manifest)
	path, digest := artifactFixture(t, string(content))
	artifact, err := isla.NewArtifact(path, digest)
	if err != nil {
		t.Fatalf("NewArtifact() error = %v", err)
	}
	return artifact
}

func mustJSON(t *testing.T, value testReleaseManifest) []byte {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return content
}
