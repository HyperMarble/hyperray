// Root paths bind each declared entry state or impossible proof to its root.
// They never permit a subset or superset of entry provenance.
package coverage

func requireExplicitRootPath(catalog *catalogs, entry RootEntry) error {
	edgeIDs, err := checkedReferences(entry.ProvenanceEdgeIDs, catalog.provenance, "root_entry.edge")
	if err != nil {
		return err
	}
	targets := make(idSet)
	for _, edgeID := range sortedIDs(edgeIDs) {
		edge := catalog.edges[edgeID]
		if edge.To.Kind != NodeModelState {
			return coverageError("root_provenance_mismatch", entry.RootID, edgeID)
		}
		if err := requireEdge(catalog, edgeID, NodeRoot, entry.RootID,
			NodeModelState, edge.To.ID); err != nil {
			return err
		}
		targets[edge.To.ID] = struct{}{}
	}
	if len(edgeIDs) != len(entry.StateIDs) || !equalIDSets(targets, stringIDSet(entry.StateIDs)) {
		return coverageError("root_provenance_mismatch", entry.RootID)
	}
	return nil
}

func requireImpossibleRootPath(catalog *catalogs, entry RootEntry) error {
	edges, err := checkedReferences(entry.ProvenanceEdgeIDs, catalog.provenance, "root_entry.edge")
	if err != nil {
		return err
	}
	if len(edges) != 1 {
		return coverageError("root_provenance_mismatch", entry.RootID)
	}
	return requireEdge(catalog, sortedIDs(edges)[0], NodeRoot, entry.RootID,
		NodeImpossiblePreconditionProof, entry.ImpossiblePreconditionProofID)
}
