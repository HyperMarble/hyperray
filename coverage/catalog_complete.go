// Reverse completeness rejects each unused independent catalog item.
// It never infers use from the existence of an identifier.
package coverage

func requireCatalogComplete(catalog catalogs) error {
	mapped := make(idSet)
	eliminated := make(idSet)
	for _, id := range sortedOperationIDs(catalog.operations) {
		if catalog.operations[id].Disposition == DispositionEliminated {
			eliminated[id] = struct{}{}
			continue
		}
		mapped[id] = struct{}{}
	}
	checks := []struct {
		code     string
		required idSet
		actual   idSet
	}{
		{"missing_operation_binding", mapped, catalog.used.operations},
		{"missing_elimination_record", eliminated, catalog.used.operations},
		{"unused_compiler_output", catalog.outputs, catalog.used.outputs},
		{"unused_image_instruction", catalog.instructions, catalog.used.instructions},
		{"unused_semantic_rule", catalog.rules, catalog.used.rules},
		{"unused_provenance_edge", catalog.provenance, catalog.used.provenance},
		{"unused_impossible_proof", catalog.impossible, catalog.used.impossible},
		{"unused_elimination_proof", catalog.proofs, catalog.used.proofs},
		{"unused_artifact", catalog.artifactIDs, catalog.used.artifacts},
	}
	for _, check := range checks {
		if missing := sortedMissingIDs(check.required, check.actual); len(missing) != 0 {
			return coverageError(check.code, missing...)
		}
	}
	return nil
}
