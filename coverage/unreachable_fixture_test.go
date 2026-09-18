// Unreachable setup adds full provenance without adding a query root.
// It never marks the disconnected state as reachable.
package coverage_test

import "github.com/HyperMarble/hyperray/coverage"

func addUnreachableEdges(request *testRequest) {
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges,
		coverage.ProvenanceEdge{ID: "edge:unreachable-output",
			From:     coverage.ProvenanceNode{Kind: coverage.NodeOperation, ID: "unreachable"},
			To:       coverage.ProvenanceNode{Kind: coverage.NodeCompilerOutput, ID: "ssa:compiled"},
			Artifact: request.Inventory.CompilerOutputs[0].Artifact},
		coverage.ProvenanceEdge{ID: "edge:add-rule-disconnected",
			From:     coverage.ProvenanceNode{Kind: coverage.NodeSemanticRule, ID: "rule:add"},
			To:       coverage.ProvenanceNode{Kind: coverage.NodeModelTransition, ID: "disconnected-step"},
			Artifact: request.Inventory.SemanticRules[0].Artifact},
	)
}

func addUnreachableBinding(request *testRequest) {
	binding := request.Inventory.MachineBindings[0]
	binding.OperationID = "unreachable"
	binding.TransitionID = "disconnected-step"
	binding.OperationToCompilerOutputEdgeID = "edge:unreachable-output"
	binding.SemanticRuleToTransitionEdgeID = "edge:add-rule-disconnected"
	request.Inventory.MachineBindings = append(request.Inventory.MachineBindings, binding)
	mapping := request.Certificate.Machine[0]
	mapping.OperationID = "unreachable"
	mapping.TransitionID = "disconnected-step"
	mapping.OperationToCompilerOutputEdgeID = "edge:unreachable-output"
	mapping.SemanticRuleToTransitionEdgeID = "edge:add-rule-disconnected"
	request.Certificate.Machine = append(request.Certificate.Machine, mapping)
}
