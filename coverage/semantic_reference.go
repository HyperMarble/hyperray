// Semantic reference checks resolve rules, transitions, and provenance.
// They never accept compiler operations in semantic-only mappings.
package coverage

func requireSemanticReferences(catalog *catalogs, binding SemanticBinding) error {
	err := requireEvidence(
		evidenceField{"semantic.operation_id", binding.OperationID},
		evidenceField{"semantic.kind", string(binding.Kind)},
		evidenceField{"semantic.semantic_rule_id", binding.SemanticRuleID},
		evidenceField{"semantic.transition_id", binding.TransitionID},
		evidenceField{"semantic.operation_to_rule", binding.OperationToSemanticRuleEdgeID},
		evidenceField{"semantic.rule_to_transition", binding.SemanticRuleToTransitionEdgeID},
	)
	if err != nil {
		return err
	}
	switch binding.Kind {
	case OperationSynthetic, OperationEnvironment:
	default:
		return coverageError("invalid_semantic_kind", string(binding.Kind))
	}
	if err := requireMappedOperation(catalog, binding.OperationID, binding.Kind); err != nil {
		return err
	}
	references := []struct {
		ids   idSet
		id    string
		label string
	}{
		{catalog.rules, binding.SemanticRuleID, "semantic_rule"},
		{catalog.transitions, binding.TransitionID, "transition"},
	}
	for _, reference := range references {
		if err := requireReference(reference.ids, reference.id, reference.label); err != nil {
			return err
		}
	}
	return requireSemanticPath(catalog, binding)
}
