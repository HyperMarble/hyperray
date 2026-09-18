// Stale-mapping tests reject arbitrary nonempty forward references.
// They never confuse nonempty text with a resolved catalog item.
package coverage_test

import "testing"

func TestStaleMappingReference(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testRequest)
	}{
		{"compiler output", func(request *testRequest) {
			request.Certificate.Machine[0].CompilerOutputID = "ssa:stale"
		}},
		{"instruction", func(request *testRequest) {
			request.Certificate.Machine[0].InstructionID = "instruction:stale"
		}},
		{"semantic rule", func(request *testRequest) {
			request.Certificate.Machine[0].SemanticRuleID = "rule:stale"
		}},
		{"provenance edge", func(request *testRequest) {
			request.Certificate.Machine[0].OperationToCompilerOutputEdgeID = "edge:stale"
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
