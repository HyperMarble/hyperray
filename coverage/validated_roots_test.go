// Validated-root tests select exact roots only through the opaque token.
// They never authorize caller-constructed state identifiers.
package coverage_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestValidatedRootsCanonicalSelection(t *testing.T) {
	request := completeRequest(t)
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	request.Inventory.RootEntries[0].StateIDs[0] = "environment"
	got, err := validated.Roots([]string{"worker-root", "main-root"})
	if err != nil {
		t.Fatalf("Roots() error = %v", err)
	}
	want := []coverage.ValidatedRoot{
		{ID: "main-root", StateIDs: []string{"start", "work"}},
		{ID: "worker-root", StateIDs: []string{"environment"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Roots() = %#v, want %#v", got, want)
	}
	got[0].StateIDs[0] = "changed"
	again, err := validated.Roots([]string{"main-root", "worker-root"})
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Errorf("second Roots() = %#v, error = %v", again, err)
	}
}

func TestValidatedRootSelectionFailures(t *testing.T) {
	request := completeRequest(t)
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	cases := []struct {
		name      string
		ids       []string
		code      string
		reference string
	}{
		{"empty", nil, "empty_root_selection", "root_ids"},
		{"unknown", []string{"missing"}, "unknown_root", "missing"},
		{"duplicate", []string{"main-root", "main-root"}, "duplicate_id", "root_selection"},
	}
	for _, testCase := range cases {
		_, err := validated.Roots(testCase.ids)
		failure := requireStructuredCode(t, err, testCase.code)
		if failure.References[0] != testCase.reference {
			t.Errorf("%s Error.References = %v", testCase.name, failure.References)
		}
	}
}

func TestZeroValidatedCoverageRejectsRoots(t *testing.T) {
	var validated coverage.ValidatedCoverage
	_, err := validated.Roots([]string{"main-root"})
	requireStructuredCode(t, err, "invalid_validated_coverage")
}
