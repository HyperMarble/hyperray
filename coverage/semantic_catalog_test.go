// Semantic catalog tests reject duplicate, empty, and wrong-kind bindings.
// They never move a synthetic operation into environment evidence.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestSemanticCatalogFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.SemanticBindings = append(request.Inventory.SemanticBindings,
		request.Inventory.SemanticBindings[0])
	requireCoverageError(t, request, "duplicate_binding")
	request = completeRequest(t)
	request.Inventory.SemanticBindings[0].SemanticRuleID = ""
	requireCoverageError(t, request, "empty_field")
	request = completeRequest(t)
	request.Inventory.SemanticBindings[0].Kind = coverage.OperationCompiler
	requireCoverageError(t, request, "invalid_semantic_kind")
}

func TestSemanticCatalogOperationKind(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.SemanticBindings[0].OperationID = "environment"
	requireCoverageError(t, request, "wrong_mapping_kind")
}

func TestSemanticCatalogPathMismatch(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.SemanticBindings[0].OperationToSemanticRuleEdgeID =
		"edge:startup-rule-transition"
	requireCoverageError(t, request, "provenance_endpoint_mismatch")
}
