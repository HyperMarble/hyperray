// Register validation owns caller data before it enters an opaque program.
// Duplicate names and values outside the declared scalar syntax are errors.
package isla

import (
	"sort"
	"strconv"
)

func validatedRegisters(values []RegisterValue) ([]RegisterValue, error) {
	result := append([]RegisterValue(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Name < result[right].Name
	})
	for index := range result {
		if err := validateRegister(result, index); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func validateRegister(values []RegisterValue, index int) error {
	value := values[index]
	if !identifier(value.Name) {
		return engineError(InvalidInput, "register name", value.Name)
	}
	if _, err := strconv.ParseUint(value.Value, 0, 64); err != nil {
		return engineError(InvalidInput, "register value", value.Value)
	}
	if index > 0 && values[index-1].Name == value.Name {
		return engineError(InvalidInput, "register name", "duplicate "+value.Name)
	}
	return nil
}

func identifier(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		digit := character >= '0' && character <= '9'
		if !letter && (index == 0 || !digit && character != '_') {
			return false
		}
	}
	return value != ""
}
