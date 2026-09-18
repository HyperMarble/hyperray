// Machine paths require all four compiler-to-model provenance hops.
// They never replace the path with one generic provenance edge.
package coverage

func requireMachinePath(catalog *catalogs, binding MachineBinding) error {
	edges := []struct {
		id       string
		fromKind ProvenanceNodeKind
		fromID   string
		toKind   ProvenanceNodeKind
		toID     string
	}{
		{binding.OperationToCompilerOutputEdgeID, NodeOperation, binding.OperationID,
			NodeCompilerOutput, binding.CompilerOutputID},
		{binding.CompilerOutputToInstructionEdgeID, NodeCompilerOutput, binding.CompilerOutputID,
			NodeImageInstruction, binding.InstructionID},
		{binding.InstructionToSemanticRuleEdgeID, NodeImageInstruction, binding.InstructionID,
			NodeSemanticRule, binding.SemanticRuleID},
		{binding.SemanticRuleToTransitionEdgeID, NodeSemanticRule, binding.SemanticRuleID,
			NodeModelTransition, binding.TransitionID},
	}
	for _, edge := range edges {
		if err := requireEdge(catalog, edge.id, edge.fromKind, edge.fromID, edge.toKind, edge.toID); err != nil {
			return err
		}
	}
	return nil
}
