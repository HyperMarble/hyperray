// Semantic paths require operation-to-rule and rule-to-transition edges.
// They never use a compiler instruction for a declared semantic operation.
package coverage

func requireSemanticPath(catalog *catalogs, binding SemanticBinding) error {
	if err := requireEdge(catalog, binding.OperationToSemanticRuleEdgeID,
		NodeOperation, binding.OperationID, NodeSemanticRule, binding.SemanticRuleID); err != nil {
		return err
	}
	return requireEdge(catalog, binding.SemanticRuleToTransitionEdgeID,
		NodeSemanticRule, binding.SemanticRuleID, NodeModelTransition, binding.TransitionID)
}
