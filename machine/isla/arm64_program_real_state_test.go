//go:build isla_integration && arm64_acceptance

// This helper parses exact ARM solver assignments and supported radices.
// It must reject duplicate keys, aliases, malformed values, and negatives.
package isla_test

import (
	"fmt"
	"strconv"
	"strings"
)

func arm64Assignments(state string) (map[string]string, error) {
	if !strings.HasSuffix(state, ";") {
		return nil, fmt.Errorf("state lacks terminal semicolon")
	}
	assignments := make(map[string]string)
	for _, entry := range strings.Split(strings.TrimSuffix(state, ";"), ";") {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid assignment %q", entry)
		}
		if _, found := assignments[parts[0]]; found {
			return nil, fmt.Errorf("duplicate assignment %q", parts[0])
		}
		assignments[parts[0]] = parts[1]
	}
	return assignments, nil
}

func arm64WitnessValue(assignments map[string]string, names []string) (uint64, error) {
	key := ""
	for _, name := range names {
		if _, found := assignments[name]; found {
			if key != "" {
				return 0, fmt.Errorf("duplicate register aliases %s and %s", key, name)
			}
			key = name
		}
	}
	if key == "" {
		return 0, fmt.Errorf("missing exact keys %v", names)
	}
	return arm64RadixValue(assignments[key])
}

func arm64RadixValue(value string) (uint64, error) {
	if strings.HasPrefix(value, "#x") {
		return strconv.ParseUint(value[2:], 16, 64)
	}
	if strings.HasPrefix(value, "#b") {
		return strconv.ParseUint(value[2:], 2, 64)
	}
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") || strings.HasPrefix(value, "0b") || strings.HasPrefix(value, "0B") || strings.HasPrefix(value, "0o") || strings.HasPrefix(value, "0O") {
		return strconv.ParseUint(value, 0, 64)
	}
	return strconv.ParseUint(value, 10, 64)
}
