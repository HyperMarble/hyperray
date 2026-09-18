// Root certificate checks compare claims with independent entry declarations.
// They never let a certificate choose its own entry-state set.
package coverage

func checkRootMappings(catalog catalogs, mappings []RootMapping) error {
	ids := make([]string, 0, len(mappings))
	byID := make(map[string]RootMapping, len(mappings))
	for _, mapping := range mappings {
		ids = append(ids, mapping.RootID)
		byID[mapping.RootID] = mapping
	}
	mappedRoots, err := uniqueIDSet(ids, "root_mapping")
	if err != nil {
		return err
	}
	for _, rootID := range sortedIDs(mappedRoots) {
		if err := checkRootMapping(catalog, byID[rootID]); err != nil {
			return err
		}
	}
	if missing := sortedMissingIDs(catalog.roots, mappedRoots); len(missing) != 0 {
		return coverageError("missing_root_mapping", missing...)
	}
	return nil
}

func checkRootMapping(catalog catalogs, mapping RootMapping) error {
	if err := requireReference(catalog.roots, mapping.RootID, "root"); err != nil {
		return err
	}
	states, err := checkedReferences(mapping.StateIDs, catalog.states, "root_mapping.state")
	if err != nil {
		return err
	}
	if mapping.ImpossiblePreconditionProofID != "" {
		if err := requireReference(catalog.impossible, mapping.ImpossiblePreconditionProofID,
			"impossible_precondition_proof"); err != nil {
			return err
		}
	}
	edges, err := checkedReferences(mapping.ProvenanceEdgeIDs, catalog.provenance, "root_mapping.edge")
	if err != nil {
		return err
	}
	expected := catalog.rootEntries[mapping.RootID]
	wantStates := stringIDSet(expected.StateIDs)
	wantEdges := stringIDSet(expected.ProvenanceEdgeIDs)
	if mapping.ImpossiblePreconditionProofID != expected.ImpossiblePreconditionProofID ||
		!equalIDSets(states, wantStates) || !equalIDSets(edges, wantEdges) {
		return coverageError("root_mapping_mismatch", mapping.RootID)
	}
	return nil
}
