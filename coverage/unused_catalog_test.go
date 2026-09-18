// Reverse-catalog tests reject valid but unused independent declarations.
// They never equate catalog membership with reconciled coverage.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestUnusedCatalogEntry(t *testing.T) {
	artifact := coverage.ArtifactReference{
		ArtifactID: "artifact:compiler",
		SHA256:     "e996bb0ea465fae70d3e3c66b3b6e02d33d2f1eb76d5958720578b6cf359cc2e",
	}
	cases := []struct {
		name string
		code string
		add  func(*coverage.CompilerInventory)
	}{
		{"output", "unused_compiler_output", func(value *coverage.CompilerInventory) {
			value.CompilerOutputs = append(value.CompilerOutputs, coverage.CompilerOutput{ID: "ssa:unused", Artifact: artifact})
		}},
		{"instruction", "unused_image_instruction", func(value *coverage.CompilerInventory) {
			value.ImageInstructions = append(value.ImageInstructions, coverage.ImageInstruction{ID: "instruction:unused", Artifact: artifact})
		}},
		{"rule", "unused_semantic_rule", func(value *coverage.CompilerInventory) {
			value.SemanticRules = append(value.SemanticRules, coverage.SemanticRule{ID: "rule:unused", Artifact: artifact})
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			testCase.add(&request.Inventory)
			requireCoverageError(t, request, testCase.code)
		})
	}
}
