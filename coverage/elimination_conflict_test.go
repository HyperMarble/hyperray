// Elimination conflict tests enforce one independent record per operation.
// They never select a record from caller slice order.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestConflictingEliminationRecords(t *testing.T) {
	request := completeRequest(t)
	firstEdge := len(request.Inventory.ProvenanceEdges)
	addEliminationClaim(&request)
	record := request.Inventory.EliminationRecords[0]
	record.ID = "elimination:optimized-two"
	record.OperationToRecordEdgeID = "edge:optimized-record-two"
	record.RecordToProofEdgeID = "edge:record-proof-two"
	record.ProofToTransitionEdgeIDs = []string{"edge:proof-compiled-step-two"}
	edges := append([]coverage.ProvenanceEdge(nil),
		request.Inventory.ProvenanceEdges[firstEdge:firstEdge+3]...)
	edges[0].ID = record.OperationToRecordEdgeID
	edges[0].To.ID = record.ID
	edges[1].ID = record.RecordToProofEdgeID
	edges[1].From.ID = record.ID
	edges[2].ID = record.ProofToTransitionEdgeIDs[0]
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges, edges...)
	request.Inventory.EliminationRecords = append(request.Inventory.EliminationRecords, record)
	requireCoverageError(t, request, "conflicting_elimination")
}
