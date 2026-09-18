// Provenance-path checks compare one edge with exact typed endpoints.
// They never accept a valid edge identifier for the wrong path position.
package coverage

func requireEdge(
	catalog *catalogs,
	edgeID string,
	fromKind ProvenanceNodeKind,
	fromID string,
	toKind ProvenanceNodeKind,
	toID string,
) error {
	if err := requireReference(catalog.provenance, edgeID, "provenance_edge"); err != nil {
		return err
	}
	edge := catalog.edges[edgeID]
	wantFrom := ProvenanceNode{Kind: fromKind, ID: fromID}
	wantTo := ProvenanceNode{Kind: toKind, ID: toID}
	if edge.From != wantFrom || edge.To != wantTo {
		return coverageError("provenance_endpoint_mismatch", edgeID, fromID, toID)
	}
	catalog.used.provenance[edgeID] = struct{}{}
	return nil
}
