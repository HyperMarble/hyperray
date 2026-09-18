// Inventory tests reject empty proof boundaries and unresolved functions.
// They never accept an artifact with no function and root inventory.
package coverage_test

import "testing"

func TestEmptyFunctionRootInventory(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Functions = nil
	request.Inventory.Roots = nil
	requireCoverageError(t, request, "empty_inventory")
}

func TestEmptyOperationInventory(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Operations = nil
	requireCoverageError(t, request, "empty_inventory")
}

func TestUnknownRootFunction(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Roots[0].FunctionID = "function:missing"
	requireCoverageError(t, request, "unknown_reference")
}

func TestMissingFunctionRoot(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Functions = append(request.Inventory.Functions,
		request.Inventory.Functions[0])
	request.Inventory.Functions[2].ID = "orphan"
	requireCoverageError(t, request, "missing_root")
}
