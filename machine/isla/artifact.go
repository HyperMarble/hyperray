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
	if path == "" || !validDigest(expectedDigest) {
		return Artifact{}, engineError(InvalidInput, path, "invalid artifact path or SHA-256 digest")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Artifact{}, engineError(InvalidInput, path, err.Error())
	}
	actualDigest, err := fileDigest(absolute)
	if err != nil {
		return Artifact{}, err
	}
	if actualDigest != expectedDigest {
		return Artifact{}, engineError(ArtifactChanged, absolute, actualDigest)
	}
	return Artifact{path: filepath.Clean(absolute), digest: actualDigest}, nil
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
		return err
	}
	if artifact.digest == "" || digest != artifact.digest {
		return engineError(ArtifactChanged, artifact.path, digest)
	}
	return nil
}
