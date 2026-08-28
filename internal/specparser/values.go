package specparser

import (
	"encoding/json"
	"fmt"
	"strings"
)

const valueSeparator = " / "

// FiniteValue is one decoded author value plus whether JSON quoting made its
// boundary explicit. Quoting matters for reserved words such as "any" and for
// an explicit singleton domain.
type FiniteValue struct {
	Value      string
	JSONQuoted bool
}

// ParseValueList decodes one strict finite-value list. Separators are the
// exact token " / " outside JSON strings. JSON strings are decoded to their
// semantic value, so paths, URLs, dates, separators, and escapes round-trip.
// Legacy bare values remain valid only when they contain no slash or quote.
func ParseValueList(raw string) (values []string, compound bool, err error) {
	decoded, compound, err := ParseFiniteValues(raw)
	if err != nil {
		return nil, false, err
	}
	values = make([]string, len(decoded))
	for index := range decoded {
		values[index] = decoded[index].Value
	}
	return values, compound, nil
}

// ParseFiniteValues is the metadata-preserving form of ParseValueList.
func ParseFiniteValues(raw string) (values []FiniteValue, compound bool, err error) {
	var tokens []string
	start := 0
	inQuote := false
	escaped := false
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inQuote = false
			}
			continue
		}
		if char == '"' {
			inQuote = true
			continue
		}
		if strings.HasPrefix(raw[index:], valueSeparator) {
			tokens = append(tokens, raw[start:index])
			index += len(valueSeparator) - 1
			start = index + 1
			compound = true
			continue
		}
		if char == '/' {
			return nil, false, fmt.Errorf("unquoted slash is ambiguous; quote the complete value as a JSON string and use %q only between values", valueSeparator)
		}
	}
	if inQuote || escaped {
		return nil, false, fmt.Errorf("unterminated JSON string in finite value list")
	}
	tokens = append(tokens, raw[start:])
	seen := map[string]struct{}{}
	for _, token := range tokens {
		value, decodeErr := decodeValueToken(token)
		if decodeErr != nil {
			return nil, false, decodeErr
		}
		if _, exists := seen[value.Value]; exists {
			return nil, false, fmt.Errorf("duplicate finite value %q", value.Value)
		}
		seen[value.Value] = struct{}{}
		values = append(values, value)
	}
	return values, compound, nil
}

func decodeValueToken(raw string) (FiniteValue, error) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return FiniteValue{}, fmt.Errorf("empty finite value")
	}
	if strings.HasPrefix(token, "\"") || strings.HasSuffix(token, "\"") {
		if !strings.HasPrefix(token, "\"") || !strings.HasSuffix(token, "\"") {
			return FiniteValue{}, fmt.Errorf("malformed JSON finite value %q", token)
		}
		var value string
		if err := json.Unmarshal([]byte(token), &value); err != nil {
			return FiniteValue{}, fmt.Errorf("invalid JSON finite value %q: %w", token, err)
		}
		if value == "" {
			return FiniteValue{}, fmt.Errorf("empty finite value")
		}
		return FiniteValue{Value: value, JSONQuoted: true}, nil
	}
	token = strings.TrimSpace(strings.ReplaceAll(token, "`", ""))
	if token == "" {
		return FiniteValue{}, fmt.Errorf("empty finite value")
	}
	if strings.ContainsAny(token, "\"/") {
		return FiniteValue{}, fmt.Errorf("ambiguous bare finite value %q; use a JSON string", token)
	}
	return FiniteValue{Value: token}, nil
}

func truncateOutsideJSONString(raw, marker string) string {
	inQuote := false
	escaped := false
	for index := 0; index+len(marker) <= len(raw); index++ {
		char := raw[index]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inQuote = false
			}
			continue
		}
		if char == '"' {
			inQuote = true
			continue
		}
		if strings.HasPrefix(raw[index:], marker) {
			return raw[:index]
		}
	}
	return raw
}
