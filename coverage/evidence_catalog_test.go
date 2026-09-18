// Evidence catalog tests reject duplicate records and stale artifact links.
// They never skip later catalogs when their own data is invalid.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestEvidenceCatalogIdentityFailures(t *testing.T) {
	cases := []struct {
		name string
		add  func(*coverage.CompilerInventory)
	}{
		{"output", func(value *coverage.CompilerInventory) {
			value.CompilerOutputs = append(value.CompilerOutputs, value.CompilerOutputs[0])
		}},
		{"instruction", func(value *coverage.CompilerInventory) {
			value.ImageInstructions = append(value.ImageInstructions, value.ImageInstructions[0])
		}},
		{"rule", func(value *coverage.CompilerInventory) {
			value.SemanticRules = append(value.SemanticRules, value.SemanticRules[0])
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			testCase.add(&request.Inventory)
			requireCoverageError(t, request, "duplicate_id")
		})
	}
}

func TestEvidenceCatalogArtifactFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.ImageInstructions[0].Artifact.ArtifactID = "artifact:missing"
	requireCoverageError(t, request, "unknown_reference")
	request = completeRequest(t)
	request.Inventory.SemanticRules[0].Artifact.SHA256 =
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	requireCoverageError(t, request, "stale_artifact")
}
