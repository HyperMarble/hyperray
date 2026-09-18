// Test support examines public structured validation errors.
// It never accepts an untyped or imprecise failure.
package model_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func requireValidation(
	t *testing.T,
	graph model.Model,
	code string,
	references ...string,
) *model.ValidationError {
	t.Helper()
	var problem *model.ValidationError
	err := model.Validate(graph)
	if !errors.As(err, &problem) {
		t.Fatalf("Validate() error = %T, want *model.ValidationError", err)
	}
	if problem.Code != code {
		t.Errorf("ValidationError.Code = %q, want %q", problem.Code, code)
	}
	if !reflect.DeepEqual(problem.References, references) {
		t.Errorf("ValidationError.References = %q, want %q", problem.References, references)
	}
	return problem
}

func requireSameValidation(
	t *testing.T,
	first model.Model,
	second model.Model,
	code string,
	references ...string,
) {
	t.Helper()
	firstProblem := requireValidation(t, first, code, references...)
	secondProblem := requireValidation(t, second, code, references...)
	if !reflect.DeepEqual(firstProblem, secondProblem) {
		t.Errorf("permuted validation = %#v, want %#v", secondProblem, firstProblem)
	}
}
