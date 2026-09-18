// Proof-claim support builds complete structural claims for boundary tests.
// It never treats structurally valid evidence as a validated theorem.
package coverage_test

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/HyperMarble/hyperray/coverage"
)

func addProofArtifact(request *testRequest, id string, content string) coverage.ArtifactReference {
	digest := sha256.Sum256([]byte(content))
	sha := hex.EncodeToString(digest[:])
	request.Inventory.Artifacts = append(request.Inventory.Artifacts,
		coverage.Artifact{ID: id, SHA256: sha, Content: []byte(content)})
	return coverage.ArtifactReference{ArtifactID: id, SHA256: sha}
}

func addEliminationClaim(request *testRequest) {
	artifact := addProofArtifact(request, "artifact:elimination", "elimination-proof")
	operation := coverage.Operation{ID: "optimized", FunctionID: "main",
		Kind: coverage.OperationCompiler, Location: "main.go:11",
		Disposition: coverage.DispositionEliminated}
	proof := coverage.EliminationProof{ID: "proof:elimination", Artifact: artifact}
	record := coverage.EliminationRecord{ID: "elimination:optimized", OperationID: operation.ID,
		ProofID: proof.ID, EquivalentTransitionIDs: []string{"compiled-step"},
		OperationToRecordEdgeID:  "edge:optimized-record",
		RecordToProofEdgeID:      "edge:record-proof",
		ProofToTransitionEdgeIDs: []string{"edge:proof-compiled-step"}}
	request.Inventory.Operations = append(request.Inventory.Operations, operation)
	request.Inventory.EliminationProofs = append(request.Inventory.EliminationProofs, proof)
	request.Inventory.EliminationRecords = append(request.Inventory.EliminationRecords, record)
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges,
		coverage.ProvenanceEdge{ID: record.OperationToRecordEdgeID,
			From: coverage.ProvenanceNode{Kind: coverage.NodeOperation, ID: operation.ID},
			To:   coverage.ProvenanceNode{Kind: coverage.NodeEliminationRecord, ID: record.ID}, Artifact: artifact},
		coverage.ProvenanceEdge{ID: record.RecordToProofEdgeID,
			From: coverage.ProvenanceNode{Kind: coverage.NodeEliminationRecord, ID: record.ID},
			To:   coverage.ProvenanceNode{Kind: coverage.NodeEliminationProof, ID: proof.ID}, Artifact: artifact},
		coverage.ProvenanceEdge{ID: record.ProofToTransitionEdgeIDs[0],
			From: coverage.ProvenanceNode{Kind: coverage.NodeEliminationProof, ID: proof.ID},
			To:   coverage.ProvenanceNode{Kind: coverage.NodeModelTransition, ID: "compiled-step"}, Artifact: artifact})
	request.Certificate.Eliminated = []coverage.EliminationMapping{{RecordID: record.ID,
		OperationID: record.OperationID, ProofID: record.ProofID,
		EquivalentTransitionIDs:  record.EquivalentTransitionIDs,
		OperationToRecordEdgeID:  record.OperationToRecordEdgeID,
		RecordToProofEdgeID:      record.RecordToProofEdgeID,
		ProofToTransitionEdgeIDs: record.ProofToTransitionEdgeIDs}}
}

func addImpossibleRootClaim(request *testRequest) {
	artifact := addProofArtifact(request, "artifact:impossible", "impossible-proof")
	proof := coverage.ImpossiblePreconditionProof{ID: "proof:impossible", Artifact: artifact}
	edge := &request.Inventory.ProvenanceEdges[2]
	edge.ID = "edge:root-worker-impossible"
	edge.To = coverage.ProvenanceNode{Kind: coverage.NodeImpossiblePreconditionProof, ID: proof.ID}
	edge.Artifact = artifact
	request.Inventory.ImpossiblePreconditionProofs = []coverage.ImpossiblePreconditionProof{proof}
	entry := &request.Inventory.RootEntries[1]
	entry.StateIDs = nil
	entry.ImpossiblePreconditionProofID = proof.ID
	entry.ProvenanceEdgeIDs = []string{edge.ID}
	mapping := &request.Certificate.Roots[1]
	mapping.StateIDs = nil
	mapping.ImpossiblePreconditionProofID = proof.ID
	mapping.ProvenanceEdgeIDs = []string{edge.ID}
}
