// Identifier checks apply the public coverage-schema alphabet to every ID.
// They never apply identifier syntax to locations or messages.
package coverage

func requireReference(ids idSet, id string, catalog string) error {
	if id == "" {
		return coverageError("empty_id", catalog)
	}
	if !validIdentifier(id) {
		return coverageError("invalid_identifier", catalog, id)
	}
	if _, exists := ids[id]; !exists {
		return coverageError("unknown_reference", catalog, id)
	}
	return nil
}

func validIdentifier(value string) bool {
	if value == "" || !asciiLetterOrDigit(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if asciiLetterOrDigit(character) || character == '.' || character == '_' ||
			character == ':' || character == '/' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func asciiLetterOrDigit(character byte) bool {
	return character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z' ||
		character >= '0' && character <= '9'
}

func checkedReferences(values []string, allowed idSet, catalog string) (idSet, error) {
	ids, err := uniqueIDSet(values, catalog)
	if err != nil {
		return nil, err
	}
	for _, id := range sortedIDs(ids) {
		if err := requireReference(allowed, id, catalog); err != nil {
			return nil, err
		}
	}
	return ids, nil
}
