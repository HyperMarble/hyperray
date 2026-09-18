// Internal test support examines defensive errors that opaque public types prevent.
// It never weakens the public constructors to make an invalid node reachable.
package circuit

import (
	"errors"
	"testing"
)

func requireInternalCode(t *testing.T, err error, code string) {
	t.Helper()
	var problem *EngineError
	if !errors.As(err, &problem) {
		t.Fatalf("error = %T, want *EngineError", err)
	}
	if problem.Code != code {
		t.Errorf("EngineError.Code = %q, want %q", problem.Code, code)
	}
}

func readNode(name string, width uint16) *bitVectorNode {
	return &bitVectorNode{
		operation: readVariable,
		variable:  Variable{name: name, width: width},
	}
}
