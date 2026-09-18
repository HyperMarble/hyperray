// Operation tests reject invalid fields, kinds, dispositions, and functions.
// They never interpret invalid data as elimination.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestOperationFailures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*coverage.Operation)
		code   string
	}{
		{"empty location", func(value *coverage.Operation) { value.Location = "" }, "empty_field"},
		{"unknown function", func(value *coverage.Operation) { value.FunctionID = "missing" }, "unknown_reference"},
		{"invalid kind", func(value *coverage.Operation) { value.Kind = "source" }, "invalid_operation_kind"},
		{"empty disposition", func(value *coverage.Operation) { value.Disposition = "" }, "invalid_disposition"},
		{"invalid disposition", func(value *coverage.Operation) { value.Disposition = "unreachable_with_proof" }, "invalid_disposition"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			testCase.change(&request.Inventory.Operations[0])
			requireCoverageError(t, request, testCase.code)
		})
	}
}

func TestEliminationRequiresCompilerOperation(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Operations[2].Disposition = coverage.DispositionEliminated
	requireCoverageError(t, request, "invalid_disposition")
}
