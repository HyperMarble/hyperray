// Digest functions bind external files to proof evidence.
// They must read the file that the external tool will receive.
package isla

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func fileDigest(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", engineError(InvalidInput, path, err.Error())
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func validDigest(digest string) bool {
	if len(digest) != sha256.Size*2 {
		return false
	}
	for _, digit := range digest {
		if !validHexDigit(digit) {
			return false
		}
	}
	return true
}

func validHexDigit(digit rune) bool {
	isNumber := digit >= '0' && digit <= '9'
	isLowercaseHex := digit >= 'a' && digit <= 'f'
	return isNumber || isLowercaseHex
}
