// Validated-coverage tests exercise the opaque public success boundary.
// They never read or construct its private snapshot.
package coverage_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestValidatedCoverageZeroValue(t *testing.T) {
	var validated coverage.ValidatedCoverage
	report, err := validated.Report()
	if report != (coverage.Report{}) {
		t.Errorf("zero Report() = %#v", report)
	}
	var failure *coverage.Error
	if !errors.As(err, &failure) {
		t.Fatalf("Report() error = %T, want *coverage.Error", err)
	}
	if failure.Code != "invalid_validated_coverage" {
		t.Errorf("Error.Code = %q", failure.Code)
	}
	if _, err := validated.Model(); err == nil {
		t.Error("zero Model() error = nil")
	}
}

func TestValidatedCoverageSnapshotIgnoresInputMutation(t *testing.T) {
	request := completeRequest(t)
	for index := range request.Model.States {
		request.Model.States[index].Values = map[string]string{
			"phase": request.Model.States[index].ID,
		}
	}
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	want, err := validated.Report()
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	for index := range request.Model.States {
		request.Model.States[index].Values["phase"] = "input-changed"
	}
	request.Model.States = nil
	request.Inventory.Functions = nil
	request.Inventory.Artifacts[0].Content[0] = 'X'
	request.Certificate.Machine = nil
	got, err := validated.Report()
	if err != nil {
		t.Fatalf("Report() after mutation error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Report() after mutation = %#v, want %#v", got, want)
	}
	first, err := validated.Model()
	if err != nil {
		t.Fatalf("Model() error = %v", err)
	}
	for _, state := range first.States {
		if state.Values["phase"] != state.ID {
			t.Errorf("stored state %q values = %v", state.ID, state.Values)
		}
	}
}
