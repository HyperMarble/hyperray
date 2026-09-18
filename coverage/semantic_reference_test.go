// Semantic reference tests reject stale operation, rule, transition, and edge IDs.
// They never continue after an unresolved semantic component.
package coverage_test

import "testing"

func TestSemanticCatalogReferenceFailures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testRequest)
	}{
		{"operation", func(value *testRequest) { value.Inventory.SemanticBindings[0].OperationID = "missing" }},
		{"rule", func(value *testRequest) { value.Inventory.SemanticBindings[0].SemanticRuleID = "missing" }},
		{"transition", func(value *testRequest) { value.Inventory.SemanticBindings[0].TransitionID = "missing" }},
		{"edge", func(value *testRequest) {
			value.Inventory.SemanticBindings[0].OperationToSemanticRuleEdgeID = "missing"
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			testCase.change(&request)
			requireCoverageError(t, request, "unknown_reference")
		})
	}
}
