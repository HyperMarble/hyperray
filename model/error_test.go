// These tests exercise constructible validation errors through the public API.
// They never depend on the order of supplied references.
package model_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestValidationErrorDescription(t *testing.T) {
	problem := model.ValidationError{Code: "bad_graph", References: []string{"second", "first"}}
	want := "model validation failed: bad_graph [first, second]"
	if got := problem.Error(); got != want {
		t.Errorf("ValidationError.Error() = %q, want %q", got, want)
	}
	wantReferences := []string{"second", "first"}
	if !reflect.DeepEqual(problem.References, wantReferences) {
		t.Errorf("Error() changed References to %q", problem.References)
	}
}

func TestValidationErrorDescriptionWithoutReferences(t *testing.T) {
	problem := model.ValidationError{Code: "bad_graph"}
	want := "model validation failed: bad_graph"
	if got := problem.Error(); got != want {
		t.Errorf("ValidationError.Error() = %q, want %q", got, want)
	}
}
