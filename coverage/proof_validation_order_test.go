// Proof-boundary tests require all structural failures to remain observable.
// They never let unsupported proof semantics mask earlier evidence errors.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestProofBoundaryFollowsArtifactValidation(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	request.Inventory.Artifacts[0].Content[0] = 'X'
	requireCoverageError(t, request, "artifact_digest_mismatch")
}

func TestProofBoundaryFollowsCatalogCompleteness(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	request.Inventory.CompilerOutputs = append(request.Inventory.CompilerOutputs,
		coverage.CompilerOutput{ID: "ssa:unused", Artifact: request.Inventory.CompilerOutputs[0].Artifact})
	requireCoverageError(t, request, "unused_compiler_output")
}

func TestProofBoundaryFollowsRootCertificate(t *testing.T) {
	request := completeRequest(t)
	addImpossibleRootClaim(&request)
	request.Certificate.Roots[1].StateIDs = []string{"environment"}
	requireCoverageError(t, request, "root_mapping_mismatch")
}
