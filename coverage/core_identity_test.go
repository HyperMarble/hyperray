// Core identity tests reject empty, duplicate, and unresolved declarations.
// They never depend on declaration slice order.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestCoreIdentityFailures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*coverage.CompilerInventory)
		code   string
	}{
		{"empty function", func(value *coverage.CompilerInventory) { value.Functions[0].ID = "" }, "empty_id"},
		{"duplicate function", func(value *coverage.CompilerInventory) {
			value.Functions = append(value.Functions, value.Functions[0])
		}, "duplicate_id"},
		{"empty root", func(value *coverage.CompilerInventory) { value.Roots[0].ID = "" }, "empty_id"},
		{"duplicate root", func(value *coverage.CompilerInventory) {
			value.Roots = append(value.Roots, value.Roots[0])
		}, "duplicate_id"},
		{"empty operation", func(value *coverage.CompilerInventory) { value.Operations[0].ID = "" }, "empty_id"},
		{"duplicate operation", func(value *coverage.CompilerInventory) {
			value.Operations = append(value.Operations, value.Operations[0])
		}, "duplicate_id"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			testCase.change(&request.Inventory)
			requireCoverageError(t, request, testCase.code)
		})
	}
}
