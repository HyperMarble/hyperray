// These tests reject incomplete constructor data before a completed report.
// They never depend on RISC-V instruction or fixture names.
package main

import (
	"bytes"
	"strings"
	"testing"
)

func validCatalog() catalog {
	value := catalog{
		DefinitionCount: 1, DefinitionKinds: []string{"function"},
		InstructionKinds: []string{"return"}, ValueKinds: []string{"literal"},
		OperationKinds: []string{"bitvector_add"}, PlaceKinds: []string{"identifier"},
		TypeKinds: []string{"fbits"}, InitializerKinds: []string{"value"},
		TypeInitializerKinds: []string{"none"}, CallTypeKinds: []string{"call"},
		CallReturnKinds: []string{"one"}, FunctionReturnKinds: []string{"plain"},
		TypeDefinitionKinds: []string{"enum"},
	}
	value.Origins = validOrigins(value)
	return value
}

func catalogWithoutTypes() catalog {
	value := validCatalog()
	value.TypeKinds = nil
	return value
}

func catalogWithEmptyType() catalog {
	value := validCatalog()
	value.TypeKinds = []string{""}
	return value
}

func catalogWithDuplicateType() catalog {
	value := validCatalog()
	value.TypeKinds = []string{"fbits", "fbits"}
	return value
}

func TestValidateCatalogAcceptsCompleteData(t *testing.T) {
	if err := validateCatalog(validCatalog()); err != nil {
		t.Errorf("validateCatalog() error = %v", err)
	}
}

func TestValidateCatalogRejectsIncompleteData(t *testing.T) {
	cases := []struct {
		name  string
		value catalog
	}{
		{"no definitions", catalog{}},
		{"missing category", catalogWithoutTypes()},
		{"empty kind", catalogWithEmptyType()},
		{"duplicate kind", catalogWithDuplicateType()},
	}
	for _, current := range cases {
		if err := validateCatalog(current.value); err == nil {
			t.Errorf("%s: validateCatalog() error = nil", current.name)
		}
	}
}

func TestOperateRejectsMissingPath(t *testing.T) {
	var output bytes.Buffer
	err := operate(nil, &output)
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("operate() error = %v", err)
	}
}
