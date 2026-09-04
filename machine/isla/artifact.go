// Artifact values identify each file that enters one Isla request.
// They must reject stale content before symbolic execution starts.
package isla

import (
	"path/filepath"
)

// Artifact binds one absolute file path to its expected SHA-256 digest.
type Artifact struct {
	path   string
	digest string
}

// NewArtifact accepts one file only when its content matches the expected digest.
func NewArtifact(path string, expectedDigest string) (Artifact, error) {
	if !filepath.IsAbs(path) || !validDigest(expectedDigest) {
		return Artifact{}, engineError(InvalidInput, path, "invalid artifact path or SHA-256 digest")
	}
	cleanPath := filepath.Clean(path)
	actualDigest, err := fileDigest(cleanPath)
	if err != nil {
		return Artifact{}, engineError(InvalidInput, cleanPath, err.Error())
	}
	if actualDigest != expectedDigest {
		return Artifact{}, engineError(ArtifactChanged, cleanPath, actualDigest)
	}
	return Artifact{path: cleanPath, digest: actualDigest}, nil
}

// Path returns the absolute artifact path.
func (artifact Artifact) Path() string {
	return artifact.path
}

// Digest returns the accepted SHA-256 digest.
func (artifact Artifact) Digest() string {
	return artifact.digest
}

func (artifact Artifact) current() error {
	digest, err := fileDigest(artifact.path)
	if err != nil {
		return engineError(ArtifactChanged, artifact.path, err.Error())
	}
	if artifact.digest == "" || digest != artifact.digest {
		return engineError(ArtifactChanged, artifact.path, digest)
	}
	return nil
}
