// Elimination paths bind operation, record, proof, and surviving transitions.
// They never replace the three typed hops with one generic edge.
package coverage

func requireEliminationPath(catalog *catalogs, record EliminationRecord, transitions idSet) error {
	if err := requireEdge(catalog, record.OperationToRecordEdgeID,
		NodeOperation, record.OperationID, NodeEliminationRecord, record.ID); err != nil {
		return err
	}
	if err := requireEdge(catalog, record.RecordToProofEdgeID,
		NodeEliminationRecord, record.ID, NodeEliminationProof, record.ProofID); err != nil {
		return err
	}
	edges, err := checkedReferences(
		record.ProofToTransitionEdgeIDs, catalog.provenance, "elimination.proof_edge")
	if err != nil {
		return err
	}
	targets := make(idSet, len(edges))
	for _, edgeID := range sortedIDs(edges) {
		edge := catalog.edges[edgeID]
		if edge.To.Kind != NodeModelTransition {
			return coverageError("elimination_provenance_mismatch", record.ID, edgeID)
		}
		if err := requireEdge(catalog, edgeID, NodeEliminationProof, record.ProofID,
			NodeModelTransition, edge.To.ID); err != nil {
			return err
		}
		targets[edge.To.ID] = struct{}{}
	}
	if len(edges) != len(transitions) || !equalIDSets(targets, transitions) {
		return coverageError("elimination_provenance_mismatch", record.ID)
	}
	return nil
}
