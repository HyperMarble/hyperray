// Completeness tests reject mapped operations and proof catalogs with no use.
// They never let independent declarations disappear from reverse checks.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestMissingOperationBinding(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Operations = append(request.Inventory.Operations, coverage.Operation{
		ID: "unmapped", Kind: coverage.OperationCompiler,
		Location: "dead.go:1", Disposition: coverage.DispositionMapped,
	})
	requireCoverageError(t, request, "missing_operation_binding")
}

func TestUnusedProofCatalogs(t *testing.T) {
	request := completeRequest(t)
	artifact := request.Inventory.CompilerOutputs[0].Artifact
	request.Inventory.ImpossiblePreconditionProofs = []coverage.ImpossiblePreconditionProof{{
		ID: "proof:unused", Artifact: artifact,
	}}
	requireCoverageError(t, request, "unused_impossible_proof")
	request = completeRequest(t)
	request.Inventory.EliminationProofs = []coverage.EliminationProof{{
		ID: "proof:unused", Artifact: request.Inventory.CompilerOutputs[0].Artifact,
	}}
	requireCoverageError(t, request, "unused_elimination_proof")
}
