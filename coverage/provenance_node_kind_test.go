// Node-kind tests exercise proof-node catalogs without trusting proof claims.
// They never accept a logical node from the wrong namespace.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestUnknownProvenanceNodeCatalogs(t *testing.T) {
	cases := []struct {
		kind coverage.ProvenanceNodeKind
		code string
	}{
		{coverage.NodeEliminationRecord, "unknown_reference"},
		{coverage.NodeEliminationProof, "unknown_reference"},
		{coverage.NodeImpossiblePreconditionProof, "unknown_reference"},
	}
	for _, testCase := range cases {
		t.Run(string(testCase.kind), func(t *testing.T) {
			request := completeRequest(t)
			request.Inventory.ProvenanceEdges[3].From = coverage.ProvenanceNode{
				Kind: testCase.kind, ID: "missing",
			}
			requireCoverageError(t, request, testCase.code)
		})
	}
}

func TestEmptyProvenanceNodeKind(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.ProvenanceEdges[3].From.Kind = ""
	requireCoverageError(t, request, "empty_field")
}
