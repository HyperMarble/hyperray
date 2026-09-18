// ARM64 reset-register validation keeps names and values canonical.
// It must retain caller ordering only after producing deterministic output.
package isla

import (
	"fmt"
	"sort"
	"strconv"
)

func validatedARM64Registers(values []RegisterValue, entry uint64, returnAddress uint64) ([]RegisterValue, error) {
	result := copyRegisterValues(values)
	sort.Slice(result, func(left int, right int) bool { return result[left].Name < result[right].Name })
	seen := make(map[string]bool, len(result))
	foundReturn := false
	for index, value := range result {
		parsed, err := validateARM64Register(value, seen, entry, returnAddress)
		if err != nil {
			return nil, err
		}
		result[index].Value = fmt.Sprintf("0x%016x", parsed)
		seen[value.Name] = true
		foundReturn = foundReturn || value.Name == "R30"
	}
	if !foundReturn {
		return nil, engineError(InvalidInput, "ARM64 return register", "R30 is required")
	}
	return result, nil
}

func validateARM64Register(value RegisterValue, seen map[string]bool, entry uint64, returnAddress uint64) (uint64, error) {
	if seen[value.Name] || !arm64RegisterName(value.Name) {
		return 0, engineError(InvalidInput, "ARM64 register", value.Name)
	}
	parsed, err := strconv.ParseUint(value.Value, 0, 64)
	if err != nil {
		return 0, engineError(InvalidInput, "ARM64 register value", value.Value)
	}
	if value.Name == "R30" && parsed != returnAddress {
		return 0, engineError(InvalidInput, "ARM64 return register", "R30 differs from return address")
	}
	if value.Name == "_PC" && parsed != entry {
		return 0, engineError(InvalidInput, "ARM64 post-reset PC", "differs from function entry")
	}
	return parsed, nil
}

func arm64RegisterName(name string) bool {
	if name == "SP_EL0" || name == "_PC" {
		return true
	}
	if len(name) < 2 || name[0] != 'R' {
		return false
	}
	value, err := strconv.ParseUint(name[1:], 10, 8)
	return err == nil && value <= 30 && name == "R"+strconv.FormatUint(value, 10)
}

func copyRegisterValues(values []RegisterValue) []RegisterValue {
	return append([]RegisterValue(nil), values...)
}
