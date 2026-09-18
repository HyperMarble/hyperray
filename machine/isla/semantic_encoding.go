// Encoding parsing compares model-executed instructions with footprint records.
// It treats encodings as data and contains no instruction-specific rules.
package isla

import (
	"encoding/hex"
	"strings"
)

func parseSemanticFootprints(lines []string) (map[string]struct{}, error) {
	encodings := make(map[string]struct{})
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if !strings.HasPrefix(value, "opcode ") {
			continue
		}
		encoding, err := semanticEncoding(value, "opcode ", ", Footprint:")
		if err != nil {
			return nil, err
		}
		if _, exists := encodings[encoding]; exists {
			return nil, semanticProtocolError("duplicate opcode footprint")
		}
		encodings[encoding] = struct{}{}
	}
	return encodings, nil
}

func semanticEncoding(value string, prefix string, suffix string) (string, error) {
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, suffix) {
		return "", semanticProtocolError("malformed instruction encoding")
	}
	encoding := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(value, prefix), suffix))
	decoded, err := hex.DecodeString(encoding)
	if err != nil || len(decoded) == 0 || len(decoded) > 4 {
		return "", semanticProtocolError("invalid instruction encoding")
	}
	return strings.Repeat("0", 8-len(encoding)) + encoding, nil
}
