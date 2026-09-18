// This test distinguishes an unused constructor group from missing data.
// It never requires each accepted profile to use the complete JIB grammar.
package main

import "testing"

func TestValidateCatalogAcceptsUnusedCategory(t *testing.T) {
	value := validCatalog()
	value.TypeInitializerKinds = []string{}
	value.Origins = originsWithoutCategory(value.Origins, "type_initializer")
	if err := validateCatalog(value); err != nil {
		t.Errorf("validateCatalog() error = %v", err)
	}
}

func originsWithoutCategory(values []origin, category string) []origin {
	result := make([]origin, 0, len(values))
	for _, value := range values {
		if value.Category != category {
			result = append(result, value)
		}
	}
	return result
}
