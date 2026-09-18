//go:build isla_integration

// Initialized writable memory must preserve the compiler's store/load behavior.
// A constant read of the original image must not produce a proof.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustInitializedMemory(t *testing.T) {
	content := compileRustExecutable(t, "initialized_memory.rs")
	initial := verifyRustExecutable(t, content, 0, 11)
	if initial.Verification.Status != isla.Proved {
		t.Fatalf("initial read result: %#v", initial.Verification)
	}
	result := verifyRustExecutable(t, content, 29, 29)
	if result.Verification.Status != isla.Proved {
		t.Fatalf("store/load result: %#v", result.Verification)
	}
	counterexample := verifyRustExecutable(t, content, 29, 11)
	if counterexample.Verification.Status != isla.Disproved {
		t.Fatalf("original byte incorrectly retained: %#v", counterexample.Verification)
	}
	t.Logf("store/load proof=%s counterexample=%s state=%s", result.Verification.Status,
		counterexample.Verification.Status, counterexample.Verification.CounterexampleState)
}
