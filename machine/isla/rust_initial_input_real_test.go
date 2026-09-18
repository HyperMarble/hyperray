//go:build isla_integration

// Typed input must change ordinary compiled behavior as its source specifies.
// The transport must not be specific to trap registers or sequential memory.
package isla_test

import (
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustTypedInputChangesResult(t *testing.T) {
	content := compileRustExecutable(t, "branch.rs")
	cases := []struct{ input, expected uint64 }{{2, 32}, {4, 16}}
	for _, example := range cases {
		boundary := rustProgramBoundary(t, content, example.input, example.expected)
		boundary.InitialRegisters = boundary.InitialRegisters[:1]
		boundary.InitialState = []isla.RegisterValue{{Name: "x10", Value: fmt.Sprintf("0x%016x", example.input)}}
		proof := verifyRustBoundary(t, content, boundary)
		if proof.Verification.Status != isla.Proved {
			t.Fatalf("typed input %d: %s", example.input, proof.Verification.Status)
		}
		boundary.NegatedAssertion = fmt.Sprintf("0:x10 = %d", example.expected)
		counterexample := verifyRustBoundary(t, content, boundary)
		assertSymbolicCounterexample(t, counterexample, example.expected)
	}
}
