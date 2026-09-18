// Origin tests reject missing, duplicated, and inconsistent occurrences.
// They never use architecture-specific instruction names.
package main

import "testing"

func TestValidateOriginsRejectsIncompleteData(t *testing.T) {
	tests := []struct {
		name   string
		change func(*catalog)
	}{
		{"empty", func(value *catalog) { value.Origins = nil }},
		{"empty id", func(value *catalog) { value.Origins[0].ID = "" }},
		{"empty category", func(value *catalog) { value.Origins[0].Category = "" }},
		{"empty kind", func(value *catalog) { value.Origins[0].Kind = "" }},
		{"duplicate id", duplicateOriginID},
		{"unknown category", func(value *catalog) { value.Origins[0].Category = "unknown" }},
		{"unknown kind", func(value *catalog) { value.Origins[0].Kind = "unknown" }},
		{"missing kind", removeOriginKind},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			value := validCatalog()
			testCase.change(&value)
			if err := validateCatalog(value); err == nil {
				t.Errorf("validateCatalog() error = nil")
			}
		})
	}
}

func duplicateOriginID(value *catalog) {
	value.Origins[1].ID = value.Origins[0].ID
}

func removeOriginKind(value *catalog) {
	value.Origins = value.Origins[1:]
}
