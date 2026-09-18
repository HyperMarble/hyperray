// Impossible-root tests require one exact root-to-proof provenance edge.
// They never let the unsupported proof boundary mask a malformed path.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestImpossibleRootPathFailures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testRequest)
		code   string
	}{
		{"unknown edge", func(request *testRequest) {
			request.Inventory.RootEntries[1].ProvenanceEdgeIDs = []string{"edge:missing"}
		}, "unknown_reference"},
		{"missing edge", func(request *testRequest) {
			request.Inventory.RootEntries[1].ProvenanceEdgeIDs = nil
		}, "root_provenance_mismatch"},
		{"wrong root", func(request *testRequest) {
			request.Inventory.ProvenanceEdges[2].From = coverage.ProvenanceNode{
				Kind: coverage.NodeRoot, ID: "main-root"}
		}, "provenance_endpoint_mismatch"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			addImpossibleRootClaim(&request)
			testCase.change(&request)
			requireCoverageError(t, request, testCase.code)
		})
	}
}
