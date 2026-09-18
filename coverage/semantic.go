// Semantic bindings declare synthetic and environment transition ownership.
// They never merge contradictory rows into set membership.
package coverage

func collectSemanticBindings(catalog *catalogs, values []SemanticBinding) error {
	ordered := orderedSemanticBindings(values)
	for index, binding := range ordered {
		if index != 0 && binding == ordered[index-1] {
			return coverageError("duplicate_binding", binding.OperationID, binding.TransitionID)
		}
		if err := requireSemanticReferences(catalog, binding); err != nil {
			return err
		}
		owner := string(binding.Kind) + ":" + binding.SemanticRuleID
		if err := claimTransition(catalog, binding.TransitionID, owner); err != nil {
			return err
		}
		catalog.semantic[binding] = struct{}{}
		catalog.used.rules[binding.SemanticRuleID] = struct{}{}
		catalog.used.operations[binding.OperationID] = struct{}{}
	}
	return nil
}
