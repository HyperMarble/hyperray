// Provenance collection fixes independent edge identities and artifacts.
// It never obtains an edge from a certificate path.
package coverage

func collectProvenance(catalog *catalogs, inventory CompilerInventory) error {
	ids := make([]string, 0, len(inventory.ProvenanceEdges))
	for _, edge := range inventory.ProvenanceEdges {
		ids = append(ids, edge.ID)
		catalog.edges[edge.ID] = edge
	}
	var err error
	if catalog.provenance, err = uniqueIDSet(ids, "provenance_edge"); err != nil {
		return err
	}
	for _, id := range sortedIDs(catalog.provenance) {
		if err := requireArtifact(catalog, catalog.edges[id].Artifact, "provenance_edge.artifact"); err != nil {
			return err
		}
	}
	return nil
}
