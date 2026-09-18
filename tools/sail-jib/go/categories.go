// Category metadata joins fixed JIB grammar groups to their used kinds.
// It never contains an instruction or source-function name.
package main

type category struct {
	key    string
	name   string
	values []string
}

func catalogCategories(value catalog) []category {
	return []category{
		{"definition", "definitions", value.DefinitionKinds},
		{"instruction", "instructions", value.InstructionKinds},
		{"value", "values", value.ValueKinds},
		{"operation", "operations", value.OperationKinds},
		{"place", "places", value.PlaceKinds},
		{"type", "types", value.TypeKinds},
		{"initializer", "initializers", value.InitializerKinds},
		{"type_initializer", "type initializers", value.TypeInitializerKinds},
		{"call_type", "call types", value.CallTypeKinds},
		{"call_return", "call returns", value.CallReturnKinds},
		{"function_return", "function returns", value.FunctionReturnKinds},
		{"type_definition", "type definitions", value.TypeDefinitionKinds},
	}
}
