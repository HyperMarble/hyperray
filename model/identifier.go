// Identifier validation implements the exact v1 schema pattern.
// It never accepts locale-specific letters or undocumented punctuation.
package model

import "strings"

const identifierInitial = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
const identifierRest = identifierInitial + "._:/-"

func identifierError(
	value string,
	emptyCode string,
	field string,
	context ...string,
) error {
	references := append([]string(nil), context...)
	references = append(references, field)
	if value == "" {
		return validationError(emptyCode, references...)
	}
	if !strings.ContainsRune(identifierInitial, rune(value[0])) {
		return validationError(codeInvalidIdentifier, append(references, value)...)
	}
	for _, character := range value[1:] {
		if !strings.ContainsRune(identifierRest, character) {
			return validationError(codeInvalidIdentifier, append(references, value)...)
		}
	}
	return nil
}

func stateReferenceError(
	transitionID string,
	stateID string,
	field string,
	unknownCode string,
	stateIDs map[string]struct{},
) error {
	if err := identifierError(stateID, codeInvalidIdentifier, field, transitionID); err != nil {
		return err
	}
	if _, exists := stateIDs[stateID]; !exists {
		return validationError(unknownCode, transitionID, stateID)
	}
	return nil
}
