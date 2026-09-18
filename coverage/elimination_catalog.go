// Elimination catalogs bind each eliminated compiler operation to one record.
// They never treat a reconciled record as a validated proof.
package coverage

func collectEliminations(catalog *catalogs, inventory CompilerInventory) error {
	ids := make([]string, 0, len(inventory.EliminationRecords))
	for _, record := range inventory.EliminationRecords {
		ids = append(ids, record.ID)
		catalog.eliminations[record.ID] = record
	}
	recordIDs, err := uniqueIDSet(ids, "elimination_record")
	if err != nil {
		return err
	}
	operationRecords := make(map[string]string)
	for _, id := range sortedIDs(recordIDs) {
		record := catalog.eliminations[id]
		if err := validateElimination(catalog, record); err != nil {
			return err
		}
		if previous, exists := operationRecords[record.OperationID]; exists {
			return coverageError("conflicting_elimination", record.OperationID, previous, id)
		}
		operationRecords[record.OperationID] = id
		catalog.unsupportedProofs["elimination:"+record.OperationID] = struct{}{}
	}
	return nil
}

func validateElimination(catalog *catalogs, record EliminationRecord) error {
	err := requireEvidence(
		evidenceField{"elimination.operation_id", record.OperationID},
		evidenceField{"elimination.proof_id", record.ProofID},
		evidenceField{"elimination.operation_to_record", record.OperationToRecordEdgeID},
		evidenceField{"elimination.record_to_proof", record.RecordToProofEdgeID},
	)
	if err != nil {
		return err
	}
	if err := requireEliminatedOperation(catalog, record.OperationID); err != nil {
		return err
	}
	if err := requireReference(catalog.proofs, record.ProofID, "elimination_proof"); err != nil {
		return err
	}
	return validateEquivalentTransitions(catalog, record)
}
