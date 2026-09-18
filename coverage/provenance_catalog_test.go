// Provenance catalog tests reject duplicate edges and invalid edge artifacts.
// They never accept untyped or unresolved logical endpoints.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestProvenanceCatalogFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges,
		request.Inventory.ProvenanceEdges[0])
	requireCoverageError(t, request, "duplicate_id")
	request = completeRequest(t)
	request.Inventory.ProvenanceEdges[0].Artifact.ArtifactID = "artifact:missing"
	requireCoverageError(t, request, "unknown_reference")
}

func TestProvenanceNodeFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.ProvenanceEdges[3].From.Kind = coverage.ProvenanceNodeKind("unknown")
	requireCoverageError(t, request, "invalid_provenance_node_kind")
	request = completeRequest(t)
	request.Inventory.ProvenanceEdges[3].To.ID = "ssa:missing"
	requireCoverageError(t, request, "unknown_reference")
}

func TestWrongProvenancePosition(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Machine[0].OperationToCompilerOutputEdgeID = "edge:output-instruction"
	requireCoverageError(t, request, "provenance_endpoint_mismatch")
}
