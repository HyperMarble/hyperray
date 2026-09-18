// Error tests expose model causes, stable messages, and zero reports.
// They never turn model validation failure into coverage output.
package coverage_test

import (
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
	"github.com/HyperMarble/hyperray/model"
)

func TestInvalidModel(t *testing.T) {
	request := completeRequest(t)
	request.Model = model.Model{}
	failure := requireCoverageError(t, request, "invalid_model")
	var modelFailure *model.ValidationError
	if !errors.As(failure, &modelFailure) {
		t.Errorf("wrapped error = %T, want *model.ValidationError", errors.Unwrap(failure))
	}
}

func TestErrorMessages(t *testing.T) {
	withReferences := coverage.Error{Code: "bad_evidence", References: []string{"first", "second"}}
	if got := withReferences.Error(); got != "coverage: bad_evidence: first, second" {
		t.Errorf("Error() = %q", got)
	}
	withoutReferences := coverage.Error{Code: "bad_evidence"}
	if got := withoutReferences.Error(); got != "coverage: bad_evidence" {
		t.Errorf("Error() = %q", got)
	}
}
