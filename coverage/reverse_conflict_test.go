// Reverse-mapping tests distinguish shared semantics from contradictory claims.
// They never make an operation identifier the exclusive transition owner.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestConflictingReverseMapping(t *testing.T) {
	request := completeRequest(t)
	artifact := request.Inventory.SemanticRules[0].Artifact
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges,
		coverage.ProvenanceEdge{ID: "edge:instruction-startup", Artifact: artifact,
			From: coverage.ProvenanceNode{Kind: coverage.NodeImageInstruction, ID: "instruction:1"},
			To:   coverage.ProvenanceNode{Kind: coverage.NodeSemanticRule, ID: "rule:startup"}},
		coverage.ProvenanceEdge{ID: "edge:startup-compiled", Artifact: artifact,
			From: coverage.ProvenanceNode{Kind: coverage.NodeSemanticRule, ID: "rule:startup"},
			To:   coverage.ProvenanceNode{Kind: coverage.NodeModelTransition, ID: "compiled-step"}},
	)
	binding := request.Inventory.MachineBindings[0]
	binding.SemanticRuleID = "rule:startup"
	binding.InstructionToSemanticRuleEdgeID = "edge:instruction-startup"
	binding.SemanticRuleToTransitionEdgeID = "edge:startup-compiled"
	request.Inventory.MachineBindings = append(request.Inventory.MachineBindings, binding)
	requireCoverageError(t, request, "conflicting_transition_binding")
}

func TestSharedTransitionSemanticIdentity(t *testing.T) {
	request := completeRequest(t)
	addSharedCompilerOperation(&request)
	report, err := checkRequest(request)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if report.MappedOperations != 4 || report.MappedTransitions != 3 {
		t.Errorf("shared counts = %d, %d", report.MappedOperations, report.MappedTransitions)
	}
}

func TestConflictingSemanticTransition(t *testing.T) {
	request := completeRequest(t)
	edge := coverage.ProvenanceEdge{
		ID:       "edge:clock-synthetic",
		From:     coverage.ProvenanceNode{Kind: coverage.NodeSemanticRule, ID: "rule:clock"},
		To:       coverage.ProvenanceNode{Kind: coverage.NodeModelTransition, ID: "synthetic-step"},
		Artifact: request.Inventory.SemanticRules[0].Artifact,
	}
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges, edge)
	request.Inventory.SemanticBindings[1].TransitionID = "synthetic-step"
	request.Inventory.SemanticBindings[1].SemanticRuleToTransitionEdgeID = edge.ID
	requireCoverageError(t, request, "conflicting_transition_binding")
}
