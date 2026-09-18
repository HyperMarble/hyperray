// Proof catalog tests reject duplicate proofs and stale proof artifacts.
// They never accept a proof ID without independent bytes.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestProofCatalogFailures(t *testing.T) {
	request := completeRequest(t)
	artifact := request.Inventory.CompilerOutputs[0].Artifact
	impossible := coverage.ImpossiblePreconditionProof{ID: "proof:impossible", Artifact: artifact}
	request.Inventory.ImpossiblePreconditionProofs = []coverage.ImpossiblePreconditionProof{impossible, impossible}
	requireCoverageError(t, request, "duplicate_id")
	request = completeRequest(t)
	elimination := coverage.EliminationProof{ID: "proof:elimination", Artifact: artifact}
	request.Inventory.EliminationProofs = []coverage.EliminationProof{elimination, elimination}
	requireCoverageError(t, request, "duplicate_id")
}

func TestProofArtifactFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.ImpossiblePreconditionProofs = []coverage.ImpossiblePreconditionProof{{
		ID: "proof:impossible", Artifact: coverage.ArtifactReference{
			ArtifactID: "artifact:missing", SHA256: request.Inventory.Artifacts[0].SHA256,
		},
	}}
	requireCoverageError(t, request, "unknown_reference")
	request = completeRequest(t)
	request.Inventory.EliminationProofs = []coverage.EliminationProof{{
		ID: "proof:elimination", Artifact: coverage.ArtifactReference{
			ArtifactID: "artifact:missing", SHA256: request.Inventory.Artifacts[0].SHA256,
		},
	}}
	requireCoverageError(t, request, "unknown_reference")
}
