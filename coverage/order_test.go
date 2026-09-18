// Ordering tests require one stable public failure for permuted input.
// They never expose slice or map iteration order.
package coverage_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestInputOrderInvariant(t *testing.T) {
	first := completeRequest(t)
	first.Inventory.Operations[0].Location = ""
	first.Inventory.Operations[2].Kind = coverage.OperationKind("bad")
	second := completeRequest(t)
	second.Inventory.Operations[0].Location = ""
	second.Inventory.Operations[2].Kind = coverage.OperationKind("bad")
	last := len(second.Inventory.Operations) - 1
	second.Inventory.Operations[0], second.Inventory.Operations[last] =
		second.Inventory.Operations[last], second.Inventory.Operations[0]
	firstError := requireCoverageError(t, first, "empty_field")
	secondError := requireCoverageError(t, second, "empty_field")
	if !reflect.DeepEqual(firstError.References, secondError.References) {
		t.Errorf("permuted references = %v, want %v", secondError.References, firstError.References)
	}
}

func TestValidInputPermutation(t *testing.T) {
	request := completeRequest(t)
	slices.Reverse(request.Model.States)
	slices.Reverse(request.Model.Transitions)
	slices.Reverse(request.Inventory.Functions)
	slices.Reverse(request.Inventory.Roots)
	slices.Reverse(request.Inventory.Operations)
	slices.Reverse(request.Inventory.Artifacts)
	slices.Reverse(request.Inventory.CompilerOutputs)
	slices.Reverse(request.Inventory.ImageInstructions)
	slices.Reverse(request.Inventory.SemanticRules)
	slices.Reverse(request.Inventory.ProvenanceEdges)
	slices.Reverse(request.Inventory.RootEntries)
	slices.Reverse(request.Inventory.MachineBindings)
	slices.Reverse(request.Inventory.SemanticBindings)
	slices.Reverse(request.Certificate.Roots)
	slices.Reverse(request.Certificate.Machine)
	slices.Reverse(request.Certificate.Synthetic)
	slices.Reverse(request.Certificate.Environment)
	if _, err := checkRequest(request); err != nil {
		t.Errorf("Check() error = %v", err)
	}
}
