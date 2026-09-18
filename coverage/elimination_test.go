// Elimination tests keep proof claims blocked without a real validator.
// They never count identifier reconciliation as an elimination proof.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestEliminationRequiresProofValidator(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	requireCoverageError(t, request, "unsupported_proof")
}

func TestIncompleteEliminationRecord(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.EliminationRecords = []coverage.EliminationRecord{{ID: "elimination:claim"}}
	requireCoverageError(t, request, "empty_field")
}
