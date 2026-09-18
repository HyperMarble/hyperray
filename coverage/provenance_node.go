// Provenance-node checks resolve each typed endpoint against its own catalog.
// They never resolve two node kinds through one shared namespace.
package coverage

func validateProvenanceNodes(catalog *catalogs) error {
	for _, edgeID := range sortedIDs(catalog.provenance) {
		edge := catalog.edges[edgeID]
		if err := requireNode(catalog, edge.From); err != nil {
			return err
		}
		if err := requireNode(catalog, edge.To); err != nil {
			return err
		}
	}
	return nil
}

func requireNode(catalog *catalogs, node ProvenanceNode) error {
	if node.Kind == "" {
		return coverageError("empty_field", "provenance_node.kind")
	}
	switch node.Kind {
	case NodeOperation:
		return requireReference(operationIDSet(catalog.operations), node.ID, string(node.Kind))
	case NodeCompilerOutput:
		return requireReference(catalog.outputs, node.ID, string(node.Kind))
	case NodeImageInstruction:
		return requireReference(catalog.instructions, node.ID, string(node.Kind))
	case NodeSemanticRule:
		return requireReference(catalog.rules, node.ID, string(node.Kind))
	case NodeModelTransition:
		return requireReference(catalog.transitions, node.ID, string(node.Kind))
	case NodeRoot:
		return requireReference(catalog.roots, node.ID, string(node.Kind))
	case NodeModelState:
		return requireReference(catalog.states, node.ID, string(node.Kind))
	case NodeEliminationRecord:
		return requireReference(eliminationIDSet(catalog.eliminations), node.ID, string(node.Kind))
	case NodeEliminationProof:
		return requireReference(catalog.proofs, node.ID, string(node.Kind))
	case NodeImpossiblePreconditionProof:
		return requireReference(catalog.impossible, node.ID, string(node.Kind))
	default:
		return coverageError("invalid_provenance_node_kind", string(node.Kind), node.ID)
	}
}
