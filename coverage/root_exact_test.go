// Root exactness tests compare certificate states with independent entries.
// They never accept a subset or superset of valid model states.
package coverage_test

import "testing"

func TestRootSubset(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Roots[0].StateIDs = []string{"start"}
	requireCoverageError(t, request, "root_mapping_mismatch")
}

func TestRootSuperset(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Roots[0].StateIDs = append(
		request.Certificate.Roots[0].StateIDs,
		"environment",
	)
	requireCoverageError(t, request, "root_mapping_mismatch")
}

func TestImpossibleRootRequiresProofValidator(t *testing.T) {
	request := completeRequest(t)
	addImpossibleRootClaim(&request)
	requireCoverageError(t, request, "unsupported_proof")
}

func TestEmptyRootWithoutProof(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.RootEntries[1].StateIDs = nil
	request.Inventory.RootEntries[1].ImpossiblePreconditionProofID = ""
	requireCoverageError(t, request, "missing_root_entry")
}
