// Elimination-certificate tests require exact independent record claims.
// They never let a certificate create or alter its own catalog record.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestMissingEliminationMapping(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	request.Certificate.Eliminated = nil
	requireCoverageError(t, request, "missing_elimination_mapping")
}

func TestInvalidEliminationMappingReference(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	request.Certificate.Eliminated[0].OperationID = "missing"
	requireCoverageError(t, request, "unknown_reference")
}

func TestEliminationMappingMismatch(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	record := &request.Inventory.EliminationRecords[0]
	edge := coverage.ProvenanceEdge{
		ID: "edge:proof-synthetic-step", Artifact: request.Inventory.EliminationProofs[0].Artifact,
		From: coverage.ProvenanceNode{Kind: coverage.NodeEliminationProof, ID: record.ProofID},
		To:   coverage.ProvenanceNode{Kind: coverage.NodeModelTransition, ID: "synthetic-step"},
	}
	record.EquivalentTransitionIDs = append(record.EquivalentTransitionIDs, "synthetic-step")
	record.ProofToTransitionEdgeIDs = append(record.ProofToTransitionEdgeIDs, edge.ID)
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges, edge)
	requireCoverageError(t, request, "elimination_mapping_mismatch")
}

func TestProofBoundaryFollowsCertificateValidation(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	request.Certificate.Machine[0].TransitionID = "missing"
	requireCoverageError(t, request, "unknown_reference")
}
