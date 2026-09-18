// Machine catalog tests reject duplicate bindings and invalid operations.
// They never accept a compiler path for another operation kind.
package coverage_test

import "testing"

func TestMachineCatalogFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.MachineBindings = append(request.Inventory.MachineBindings,
		request.Inventory.MachineBindings[0])
	requireCoverageError(t, request, "duplicate_binding")
	request = completeRequest(t)
	request.Inventory.MachineBindings[0].CompilerOutputID = ""
	requireCoverageError(t, request, "empty_field")
	request = completeRequest(t)
	request.Inventory.MachineBindings[0].OperationID = "operation:missing"
	requireCoverageError(t, request, "unknown_reference")
}

func TestMachineOperationFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.MachineBindings[0].OperationID = "synthetic"
	requireCoverageError(t, request, "wrong_mapping_kind")
}

func TestMissingTransitionBinding(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.SemanticBindings = request.Inventory.SemanticBindings[:1]
	requireCoverageError(t, request, "missing_transition_binding")
}
