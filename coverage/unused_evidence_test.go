// Reverse-evidence tests cover unused provenance and artifact declarations.
// They never let a stale evidence artifact survive reconciliation.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestUnusedProvenanceEdge(t *testing.T) {
	request := completeRequest(t)
	edge := request.Inventory.ProvenanceEdges[0]
	edge.ID = "edge:unused"
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges, edge)
	requireCoverageError(t, request, "unused_provenance_edge")
}

func TestUnusedArtifact(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Artifacts = append(request.Inventory.Artifacts, coverage.Artifact{
		ID:      "artifact:unused",
		SHA256:  "febe1d741b49e5a9c31526728d8c5134a803adfc4c04c4f052673722ed85597e",
		Content: []byte("unused"),
	})
	requireCoverageError(t, request, "unused_artifact")
}
