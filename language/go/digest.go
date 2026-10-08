// Purpose: names a file by the hash of its bytes.
// Never:   hashes a path or a name instead of the file's content.
package goadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// digestOf reads the file and returns its path with the SHA-256 of its bytes.
func digestOf(path string) (FileDigest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FileDigest{}, unreadable(path, err)
	}
	sum := sha256.Sum256(content)
	return FileDigest{Path: path, Sha256: hex.EncodeToString(sum[:])}, nil
}
