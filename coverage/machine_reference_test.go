// Machine reference tests reject stale output, instruction, rule, and transition IDs.
// They never continue after an unresolved path component.
package coverage_test

import "testing"

func TestMachineCatalogReferenceFailures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testRequest)
	}{
		{"output", func(value *testRequest) { value.Inventory.MachineBindings[0].CompilerOutputID = "missing" }},
		{"instruction", func(value *testRequest) { value.Inventory.MachineBindings[0].InstructionID = "missing" }},
		{"rule", func(value *testRequest) { value.Inventory.MachineBindings[0].SemanticRuleID = "missing" }},
		{"transition", func(value *testRequest) { value.Inventory.MachineBindings[0].TransitionID = "missing" }},
		{"edge", func(value *testRequest) {
			value.Inventory.MachineBindings[0].OperationToCompilerOutputEdgeID = "missing"
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			testCase.change(&request)
			requireCoverageError(t, request, "unknown_reference")
		})
	}
}

func TestMachineCatalogPathMismatch(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.MachineBindings[0].OperationToCompilerOutputEdgeID = "edge:output-instruction"
	requireCoverageError(t, request, "provenance_endpoint_mismatch")
}
