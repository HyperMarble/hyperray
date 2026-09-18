// Error support examines public structured engine errors from public calls.
// It never accepts an untyped or partially identified error.
package circuit_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func requireEngineError(
	t *testing.T,
	err error,
	code string,
	references ...string,
) *circuit.EngineError {
	t.Helper()
	var problem *circuit.EngineError
	if !errors.As(err, &problem) {
		t.Fatalf("error = %T, want *circuit.EngineError", err)
	}
	if problem.Code != code {
		t.Errorf("EngineError.Code = %q, want %q", problem.Code, code)
	}
	if !reflect.DeepEqual(problem.References, references) {
		t.Errorf("EngineError.References = %q, want %q", problem.References, references)
	}
	wantText := code + ": "
	if problem.Error()[:len(wantText)] != wantText {
		t.Errorf("EngineError.Error() = %q", problem.Error())
	}
	return problem
}

func requireEngineCode(t *testing.T, err error, code string) *circuit.EngineError {
	t.Helper()
	var problem *circuit.EngineError
	if !errors.As(err, &problem) {
		t.Fatalf("error = %T, want *circuit.EngineError", err)
	}
	if problem.Code != code {
		t.Errorf("EngineError.Code = %q, want %q", problem.Code, code)
	}
	return problem
}
