// Root catalog tests reject missing, duplicate, and contradictory entries.
// They never allow one root declaration to cover another.
package coverage_test

import "testing"

func TestRootCatalogIdentityFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.RootEntries = append(request.Inventory.RootEntries,
		request.Inventory.RootEntries[0])
	requireCoverageError(t, request, "duplicate_id")
	request = completeRequest(t)
	request.Inventory.RootEntries = request.Inventory.RootEntries[:1]
	requireCoverageError(t, request, "missing_root_entry")
	request = completeRequest(t)
	request.Inventory.RootEntries[0].RootID = "root:missing"
	requireCoverageError(t, request, "unknown_reference")
}

func TestRootCatalogContentFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.RootEntries[1].ImpossiblePreconditionProofID = "proof:claim"
	request.Inventory.RootEntries[1].StateIDs = []string{"environment"}
	requireCoverageError(t, request, "conflicting_root_entry")
	request = completeRequest(t)
	request.Inventory.RootEntries[0].StateIDs[0] = "state:missing"
	requireCoverageError(t, request, "unknown_reference")
	request = completeRequest(t)
	request.Inventory.RootEntries[1].StateIDs = nil
	request.Inventory.RootEntries[1].ImpossiblePreconditionProofID = "proof:missing"
	requireCoverageError(t, request, "unknown_reference")
}
