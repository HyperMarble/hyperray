// Strict JSON helpers reject duplicate object keys before typed decoding.
// The manifest identity must have one authoritative value per field.
package isla

import (
	"bytes"
	"encoding/json"
)

func rejectDuplicateObjectKeys(content []byte, subject string) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	opening, err := decoder.Token()
	if err != nil {
		return releaseError(subject + " has invalid JSON")
	}
	delimiter, ok := opening.(json.Delim)
	if !ok || delimiter != '{' {
		return nil
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return releaseError(subject + " has invalid JSON")
		}
		name, ok := key.(string)
		if !ok {
			return releaseError(subject + " has an invalid object key")
		}
		if _, found := seen[name]; found {
			return releaseError(subject + " has duplicate key " + name)
		}
		seen[name] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return releaseError(subject + " has invalid JSON")
		}
	}
	if _, err := decoder.Token(); err != nil {
		return releaseError(subject + " has invalid JSON")
	}
	return nil
}
