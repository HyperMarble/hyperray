// Hash helpers compare actual referenced bytes with build evidence.
// Caller-supplied digest fields never override a measured value.
package compilercatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func hashMatches(content []byte, expected string, name string) error {
	actual := digestBytes(content)
	if actual != expected {
		return fmt.Errorf("stale %s artifact: expected %s, got %s", name, expected, actual)
	}
	return nil
}

func sizeMatches(content []byte, expected uint64, name string) error {
	actual := uint64(len(content))
	if actual != expected {
		return fmt.Errorf("%s artifact size mismatch: expected %d, got %d", name, expected, actual)
	}
	return nil
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func digestJSON(content []byte) (string, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return digestBytes(canonical), nil
}
