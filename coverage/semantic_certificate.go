// Semantic certificate checks keep synthetic and environment claims separate.
// They never accept an extra or omitted semantic claim.
package coverage

func checkSemanticMappings(catalog catalogs, mappings []SemanticMapping, kind OperationKind) error {
	values := make([]SemanticBinding, 0, len(mappings))
	for _, mapping := range mappings {
		values = append(values, SemanticBinding{
			OperationID: mapping.OperationID, Kind: kind, SemanticRuleID: mapping.SemanticRuleID, TransitionID: mapping.TransitionID,
			OperationToSemanticRuleEdgeID:  mapping.OperationToSemanticRuleEdgeID,
			SemanticRuleToTransitionEdgeID: mapping.SemanticRuleToTransitionEdgeID,
		})
	}
	ordered := orderedSemanticBindings(values)
	seen := make(map[SemanticBinding]struct{}, len(ordered))
	for index, binding := range ordered {
		if index != 0 && binding == ordered[index-1] {
			return coverageError("duplicate_evidence", binding.OperationID, binding.TransitionID)
		}
		if err := requireSemanticReferences(&catalog, binding); err != nil {
			return err
		}
		if _, exists := catalog.semantic[binding]; !exists {
			return coverageError("semantic_mapping_mismatch", binding.OperationID, binding.TransitionID)
		}
		seen[binding] = struct{}{}
	}
	missing := make([]string, 0)
	declared := make([]SemanticBinding, 0, len(catalog.semantic))
	for binding := range catalog.semantic {
		if binding.Kind == kind {
			declared = append(declared, binding)
		}
	}
	for _, binding := range orderedSemanticBindings(declared) {
		if _, exists := seen[binding]; !exists {
			missing = append(missing, binding.OperationID+":"+binding.TransitionID)
		}
	}
	if len(missing) != 0 {
		return coverageError("missing_semantic_mapping", missing...)
	}
	return nil
}
