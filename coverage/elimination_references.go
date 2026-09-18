// Elimination references resolve the proof and every surviving transition.
// They never let reachability remove an eliminated operation.
package coverage

func requireEliminatedOperation(catalog *catalogs, id string) error {
	if err := requireReference(operationIDSet(catalog.operations), id, "operation"); err != nil {
		return err
	}
	operation := catalog.operations[id]
	if operation.Kind != OperationCompiler || operation.Disposition != DispositionEliminated {
		return coverageError("wrong_operation_disposition", id, string(operation.Disposition))
	}
	return nil
}

func validateEquivalentTransitions(catalog *catalogs, record EliminationRecord) error {
	if len(record.EquivalentTransitionIDs) == 0 {
		return coverageError("empty_field", "elimination.equivalent_transition_ids", record.ID)
	}
	transitions, err := checkedReferences(
		record.EquivalentTransitionIDs, catalog.transitions, "elimination.transition")
	if err != nil {
		return err
	}
	if err := requireEliminationPath(catalog, record, transitions); err != nil {
		return err
	}
	catalog.used.proofs[record.ProofID] = struct{}{}
	catalog.used.operations[record.OperationID] = struct{}{}
	return nil
}
