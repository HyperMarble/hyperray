// Identifier rules keep generated SMT symbols separate from caller text.
// They never quote unsupported characters into a solver command.
package circuit

import "strings"

const identifierInitial = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const identifierRest = identifierInitial + "0123456789._:/-"

func identifierError(value string, field string) error {
	if value == "" {
		return engineError("empty_identifier", field)
	}
	if !strings.ContainsRune(identifierInitial, rune(value[0])) {
		return engineError("invalid_identifier", field, value)
	}
	for _, character := range value[1:] {
		if !strings.ContainsRune(identifierRest, character) {
			return engineError("invalid_identifier", field, value)
		}
	}
	return nil
}
