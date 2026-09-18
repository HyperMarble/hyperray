// Scalar witness values are concrete numbers or booleans from the tool.
// A symbolic identifier must never masquerade as a concrete counterexample.
package isla

import (
	"math/big"
	"strings"
)

func concreteState(state string) bool {
	if !strings.HasSuffix(state, ";") {
		return false
	}
	assignments := strings.Split(strings.TrimSuffix(state, ";"), ";")
	seen := make(map[string]bool, len(assignments))
	for _, assignment := range assignments {
		name, value, found := strings.Cut(assignment, "=")
		if !found || name == "" || seen[name] || !concreteScalar(value) {
			return false
		}
		seen[name] = true
	}
	return true
}

func concreteScalar(value string) bool {
	if value == "true" || value == "false" {
		return true
	}
	number := new(big.Int)
	if strings.HasPrefix(value, "#x") || strings.HasPrefix(value, "#b") {
		value = "0" + value[1:]
	}
	_, valid := number.SetString(value, 0)
	return valid
}
