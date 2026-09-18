// The catalog decoder reads one constructor census from the Sail plug-in.
// It never accepts a missing or malformed constructor category.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type catalog struct {
	DefinitionCount      int      `json:"definition_count"`
	Origins              []origin `json:"origins"`
	DefinitionKinds      []string `json:"definition_kinds"`
	InstructionKinds     []string `json:"instruction_kinds"`
	ValueKinds           []string `json:"value_kinds"`
	OperationKinds       []string `json:"operation_kinds"`
	PlaceKinds           []string `json:"place_kinds"`
	TypeKinds            []string `json:"type_kinds"`
	InitializerKinds     []string `json:"initializer_kinds"`
	TypeInitializerKinds []string `json:"type_initializer_kinds"`
	CallTypeKinds        []string `json:"call_type_kinds"`
	CallReturnKinds      []string `json:"call_return_kinds"`
	FunctionReturnKinds  []string `json:"function_return_kinds"`
	TypeDefinitionKinds  []string `json:"type_definition_kinds"`
}

func readCatalog(path string) (catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return catalog{}, fmt.Errorf("read JIB catalog: %w", err)
	}
	var result catalog
	if err := json.Unmarshal(data, &result); err != nil {
		return catalog{}, fmt.Errorf("decode JIB catalog: %w", err)
	}
	return result, nil
}
