// Root-selection ordering makes unknown-root failures independent of input.
// It never reports the caller's arbitrary first unknown root.
package coverage_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestValidatedRootErrorOrder(t *testing.T) {
	request := completeRequest(t)
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	_, firstError := validated.Roots([]string{"z-missing", "a-missing"})
	_, secondError := validated.Roots([]string{"a-missing", "z-missing"})
	first := requireStructuredCode(t, firstError, "unknown_root")
	second := requireStructuredCode(t, secondError, "unknown_root")
	if !reflect.DeepEqual(first.References, second.References) || first.References[0] != "a-missing" {
		t.Errorf("unknown-root references = %v and %v", first.References, second.References)
	}
}
