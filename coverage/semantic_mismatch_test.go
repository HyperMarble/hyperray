// Semantic mismatch tests reject valid path pieces in the wrong complete row.
// They never reduce row equality to transition membership.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestSemanticCertificateMismatch(t *testing.T) {
	request := completeRequest(t)
	addSharedSyntheticOperation(&request)
	edge := request.Inventory.ProvenanceEdges[8]
	edge.ID = "edge:startup-transition-copy"
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges, edge)
	request.Inventory.SemanticBindings[2].SemanticRuleToTransitionEdgeID = edge.ID
	request.Certificate.Synthetic[0].SemanticRuleToTransitionEdgeID = edge.ID
	requireCoverageError(t, request, "semantic_mapping_mismatch")
}

func addSharedSyntheticOperation(request *testRequest) {
	request.Inventory.Operations = append(request.Inventory.Operations, coverage.Operation{
		ID: "synthetic-alias", Kind: coverage.OperationSynthetic,
		Location: "linker:alias", Disposition: coverage.DispositionMapped,
	})
	edge := coverage.ProvenanceEdge{
		ID:       "edge:synthetic-alias-rule",
		From:     coverage.ProvenanceNode{Kind: coverage.NodeOperation, ID: "synthetic-alias"},
		To:       coverage.ProvenanceNode{Kind: coverage.NodeSemanticRule, ID: "rule:startup"},
		Artifact: request.Inventory.SemanticRules[0].Artifact,
	}
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges, edge)
	binding := request.Inventory.SemanticBindings[0]
	binding.OperationID = "synthetic-alias"
	binding.OperationToSemanticRuleEdgeID = edge.ID
	request.Inventory.SemanticBindings = append(request.Inventory.SemanticBindings, binding)
	mapping := request.Certificate.Synthetic[0]
	mapping.OperationID = "synthetic-alias"
	mapping.OperationToSemanticRuleEdgeID = edge.ID
	request.Certificate.Synthetic = append(request.Certificate.Synthetic, mapping)
}
