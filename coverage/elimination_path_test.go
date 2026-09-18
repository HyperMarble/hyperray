// Elimination-path tests require every exact typed provenance hop.
// They never collapse proof paths into identifier membership.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestEliminationPathFailures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testRequest, int)
		code   string
	}{
		{"operation edge", func(request *testRequest, first int) {
			request.Inventory.ProvenanceEdges[first].To.ID = "elimination:other"
		}, "provenance_endpoint_mismatch"},
		{"record edge", func(request *testRequest, first int) {
			request.Inventory.ProvenanceEdges[first+1].From.ID = "elimination:other"
		}, "provenance_endpoint_mismatch"},
		{"missing proof edge", func(request *testRequest, _ int) {
			request.Inventory.EliminationRecords[0].ProofToTransitionEdgeIDs = []string{"edge:missing"}
		}, "unknown_reference"},
		{"wrong target kind", func(request *testRequest, first int) {
			request.Inventory.ProvenanceEdges[first+2].To.Kind = coverage.NodeModelState
		}, "elimination_provenance_mismatch"},
		{"wrong proof source", func(request *testRequest, first int) {
			request.Inventory.ProvenanceEdges[first+2].From.ID = "proof:other"
		}, "provenance_endpoint_mismatch"},
		{"wrong target set", func(request *testRequest, _ int) {
			request.Inventory.EliminationRecords[0].EquivalentTransitionIDs = []string{"synthetic-step"}
		}, "elimination_provenance_mismatch"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			first := len(request.Inventory.ProvenanceEdges)
			addEliminationClaim(&request)
			testCase.change(&request, first)
			requireCoverageError(t, request, testCase.code)
		})
	}
}
