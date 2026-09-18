// Elimination claim tests validate row IDs before blocking unsupported proofs.
// They never return a coverage report for an elimination claim.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestEliminationCertificateStaleRecord(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Eliminated = []coverage.EliminationMapping{{RecordID: "elimination:claim"}}
	requireCoverageError(t, request, "unknown_reference")
}

func TestEliminationCertificateIdentifierFailures(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Eliminated = []coverage.EliminationMapping{{}}
	requireCoverageError(t, request, "empty_id")
	request = completeRequest(t)
	claim := coverage.EliminationMapping{RecordID: "elimination:claim"}
	request.Certificate.Eliminated = []coverage.EliminationMapping{claim, claim}
	requireCoverageError(t, request, "duplicate_id")
}
