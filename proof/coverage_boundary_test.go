// Coverage-boundary tests prove that only a minted complete capability can enter proof.
// They never pass a caller-created report as authorization.
package proof_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
	"github.com/HyperMarble/hyperray/proof"
)

func TestIncompleteCoverageIsError(t *testing.T) {
	request := loadFixture(t)
	request.Certificate.Machine = nil
	validated, coverageErr := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if coverageErr == nil {
		t.Fatal("coverage.Check() error = nil")
	}
	result, err := proof.Check(validated, safeQuery("main-root"))
	failure := requireProofError(t, err, "invalid_coverage")
	if result.Verdict != "" || len(failure.References) != 1 {
		t.Errorf("result = %#v, error = %#v", result, failure)
	}
}

func TestZeroCoverageCapabilityIsError(t *testing.T) {
	var validated coverage.ValidatedCoverage
	result, err := proof.Check(validated, safeQuery("main-root"))
	requireProofError(t, err, "invalid_coverage")
	if !reflect.DeepEqual(result, proof.Result{}) {
		t.Errorf("proof.Check() = %#v", result)
	}
}

func requireProofError(t *testing.T, err error, code string) *proof.Error {
	t.Helper()
	var failure *proof.Error
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want *proof.Error", err)
	}
	if failure.Code != code {
		t.Errorf("Error.Code = %q, want %q", failure.Code, code)
	}
	return failure
}

func safeQuery(rootID string) proof.Query {
	return proof.Query{
		RootIDs: []string{rootID},
		Requirement: proof.Requirement{
			ID: "no-environment", Kind: proof.RequirementSafety,
			BadStateIDs: []string{"environment"},
		},
	}
}
