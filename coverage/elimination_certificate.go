// Elimination certificates must equal independent elimination records.
// They never turn structural reconciliation into a proof verdict.
package coverage

func checkEliminations(catalog catalogs, mappings []EliminationMapping) error {
	ids := make([]string, 0, len(mappings))
	byID := make(map[string]EliminationMapping, len(mappings))
	for _, mapping := range mappings {
		ids = append(ids, mapping.RecordID)
		byID[mapping.RecordID] = mapping
	}
	mapped, err := uniqueIDSet(ids, "elimination_mapping")
	if err != nil {
		return err
	}
	for _, id := range sortedIDs(mapped) {
		if err := checkElimination(catalog, byID[id]); err != nil {
			return err
		}
	}
	if missing := sortedMissingIDs(eliminationIDSet(catalog.eliminations), mapped); len(missing) != 0 {
		return coverageError("missing_elimination_mapping", missing...)
	}
	return nil
}

func checkElimination(catalog catalogs, mapping EliminationMapping) error {
	if err := requireReference(eliminationIDSet(catalog.eliminations),
		mapping.RecordID, "elimination_record"); err != nil {
		return err
	}
	actual := EliminationRecord{
		ID: mapping.RecordID, OperationID: mapping.OperationID, ProofID: mapping.ProofID,
		EquivalentTransitionIDs:  mapping.EquivalentTransitionIDs,
		OperationToRecordEdgeID:  mapping.OperationToRecordEdgeID,
		RecordToProofEdgeID:      mapping.RecordToProofEdgeID,
		ProofToTransitionEdgeIDs: mapping.ProofToTransitionEdgeIDs,
	}
	if err := validateElimination(&catalog, actual); err != nil {
		return err
	}
	expected := catalog.eliminations[mapping.RecordID]
	if actual.OperationID != expected.OperationID || actual.ProofID != expected.ProofID ||
		actual.OperationToRecordEdgeID != expected.OperationToRecordEdgeID ||
		actual.RecordToProofEdgeID != expected.RecordToProofEdgeID ||
		!equalIDSets(stringIDSet(actual.EquivalentTransitionIDs),
			stringIDSet(expected.EquivalentTransitionIDs)) ||
		!equalIDSets(stringIDSet(actual.ProofToTransitionEdgeIDs),
			stringIDSet(expected.ProofToTransitionEdgeIDs)) {
		return coverageError("elimination_mapping_mismatch", mapping.RecordID)
	}
	return nil
}
