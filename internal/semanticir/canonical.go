package semanticir

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// CanonicalJSON returns the compact deterministic JSON encoding used for IR
// hashes and certificates. Go's JSON encoder orders string map keys; disabling
// HTML escaping avoids environment-dependent presentation rewrites.
func CanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("canonical JSON: %w", err)
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}

// Digest returns the sha256 digest of CanonicalJSON(value).
func Digest(value any) (string, error) {
	encoded, err := CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	return DigestBytes(encoded), nil
}

// DigestBytes returns Ray's normalized SHA-256 representation.
func DigestBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ValidDigest reports whether digest has Ray's normalized SHA-256 syntax.
func ValidDigest(digest string) bool {
	return digestPattern.MatchString(digest)
}

// VerifyArtifact checks both the shape of a frozen reference and its content.
func VerifyArtifact(artifact ArtifactRef, content []byte) error {
	if artifact.ID == "" {
		return fmt.Errorf("artifact ID is empty")
	}
	if artifact.Kind == "" {
		return fmt.Errorf("artifact %q kind is empty", artifact.ID)
	}
	if artifact.Path == "" {
		return fmt.Errorf("artifact %q path is empty", artifact.ID)
	}
	if !ValidDigest(artifact.Digest) {
		return fmt.Errorf("artifact %q has invalid digest %q", artifact.ID, artifact.Digest)
	}
	if actual := DigestBytes(content); actual != artifact.Digest {
		return fmt.Errorf("artifact %q digest mismatch: frozen %s, actual %s", artifact.ID, artifact.Digest, actual)
	}
	return nil
}
