// Root path tests reject wrong endpoint kinds, roots, and edge cardinality.
// They never collapse duplicate paths into a state set.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestExplicitRootPathFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.ProvenanceEdges[0].To.Kind = coverage.NodeModelTransition
	requireCoverageError(t, request, "root_provenance_mismatch")
	request = completeRequest(t)
	request.Inventory.ProvenanceEdges[0].From.ID = "worker-root"
	requireCoverageError(t, request, "provenance_endpoint_mismatch")
}

func TestDuplicateRootStatePath(t *testing.T) {
	request := completeRequest(t)
	edge := request.Inventory.ProvenanceEdges[0]
	edge.ID = "edge:root-main-start-copy"
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges, edge)
	request.Inventory.RootEntries[0].ProvenanceEdgeIDs = append(
		request.Inventory.RootEntries[0].ProvenanceEdgeIDs,
		edge.ID,
	)
	requireCoverageError(t, request, "root_provenance_mismatch")
}

func TestRootPathCardinality(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.RootEntries[1].ProvenanceEdgeIDs = nil
	requireCoverageError(t, request, "root_provenance_mismatch")
}

func TestMissingRootPathEdge(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.RootEntries[0].ProvenanceEdgeIDs[0] = "edge:missing"
	requireCoverageError(t, request, "unknown_reference")
}
