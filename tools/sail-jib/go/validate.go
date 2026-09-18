// Catalog validation makes the JIB census result explicit and deterministic.
// It never repairs missing, duplicate, or unordered constructor data.
package main

import "fmt"

func validateCatalog(value catalog) error {
	if value.DefinitionCount < 1 {
		return fmt.Errorf("JIB catalog has %d definitions", value.DefinitionCount)
	}
	for _, current := range catalogCategories(value) {
		if err := validateCategory(current); err != nil {
			return err
		}
	}
	return validateOrigins(value)
}

func validateCategory(current category) error {
	if current.values == nil {
		return fmt.Errorf("JIB catalog category %q is missing", current.name)
	}
	previous := ""
	for position, value := range current.values {
		if value == "" {
			return fmt.Errorf("JIB catalog category %q has an empty kind", current.name)
		}
		if position > 0 && value <= previous {
			return fmt.Errorf("JIB catalog category %q is not unique and sorted", current.name)
		}
		previous = value
	}
	return nil
}
