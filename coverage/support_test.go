// Test support loads canonical public JSON and inspects structured failures.
// It never builds a private shortcut around the public Check API.
package coverage_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
	"github.com/HyperMarble/hyperray/model"
)

type testRequest struct {
	Model       model.Model                `json:"model"`
	Inventory   coverage.CompilerInventory `json:"inventory"`
	Certificate coverage.Certificate       `json:"certificate"`
}

func completeRequest(t *testing.T) testRequest {
	t.Helper()
	contents, err := os.ReadFile("fixtures/complete.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	var request testRequest
	if err := json.Unmarshal(contents, &request); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return request
}

func checkRequest(request testRequest) (coverage.Report, error) {
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		return coverage.Report{}, err
	}
	return validated.Report()
}

func requireCoverageError(t *testing.T, request testRequest, code string) *coverage.Error {
	t.Helper()
	result, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if !reflect.DeepEqual(result, coverage.ValidatedCoverage{}) {
		t.Errorf("Check() result on error = %#v", result)
	}
	var failure *coverage.Error
	if !errors.As(err, &failure) {
		t.Fatalf("Check() error = %T, want *coverage.Error", err)
	}
	if failure.Code != code {
		t.Errorf("Error.Code = %q, want %q", failure.Code, code)
	}
	if len(failure.References) == 0 {
		t.Error("Error.References is empty")
	}
	return failure
}

func requireStructuredCode(t *testing.T, err error, code string) *coverage.Error {
	t.Helper()
	var failure *coverage.Error
	if !errors.As(err, &failure) {
		t.Fatalf("error = %T, want *coverage.Error", err)
	}
	if failure.Code != code {
		t.Errorf("Error.Code = %q, want %q", failure.Code, code)
	}
	if len(failure.References) == 0 {
		t.Error("Error.References is empty")
	}
	return failure
}
