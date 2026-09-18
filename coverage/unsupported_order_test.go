// Unsupported-proof ordering is stable across proof-bearing input order.
// It never reports whichever proof claim the caller lists first.
package coverage_test

import (
	"reflect"
	"slices"
	"testing"
)

func TestUnsupportedProofOrderInvariant(t *testing.T) {
	first := completeRequest(t)
	addEliminationClaim(&first)
	addImpossibleRootClaim(&first)
	firstError := requireCoverageError(t, first, "unsupported_proof")
	second := completeRequest(t)
	addEliminationClaim(&second)
	addImpossibleRootClaim(&second)
	slices.Reverse(second.Inventory.Operations)
	slices.Reverse(second.Inventory.Artifacts)
	slices.Reverse(second.Inventory.ProvenanceEdges)
	slices.Reverse(second.Inventory.RootEntries)
	slices.Reverse(second.Certificate.Roots)
	secondError := requireCoverageError(t, second, "unsupported_proof")
	want := []string{"elimination:optimized", "impossible_precondition:worker-root"}
	if !reflect.DeepEqual(firstError.References, want) ||
		!reflect.DeepEqual(secondError.References, want) {
		t.Errorf("unsupported references = %v and %v", firstError.References, secondError.References)
	}
}
