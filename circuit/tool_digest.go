// Executable digests distinguish tools that share a path or version string.
// A proposal never records an unreadable executable as an exact identity.
package circuit

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func executableDigest(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", engineError("tool_digest_error", path, err.Error())
	}
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
