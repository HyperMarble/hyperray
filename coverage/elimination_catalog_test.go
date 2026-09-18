// Elimination catalog tests validate identities before blocking proof claims.
// They never accept an empty or duplicate record identifier.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestEliminationCatalogFailures(t *testing.T) {
	request := completeRequest(t)
	record := coverage.EliminationRecord{ID: "elimination:claim"}
	request.Inventory.EliminationRecords = []coverage.EliminationRecord{record, record}
	requireCoverageError(t, request, "duplicate_id")
	request = completeRequest(t)
	request.Inventory.EliminationRecords = []coverage.EliminationRecord{{}}
	requireCoverageError(t, request, "empty_id")
}
