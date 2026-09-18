//go:build isla_integration

// Different Rust programs use the same public executable verification API.
// An untaken branch must not disappear from the static instruction inventory.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

type rustMachineCase struct {
	name          string
	source        string
	input         uint64
	expected      uint64
	hasUnobserved bool
}

func TestRealRustExecutablePrograms(t *testing.T) {
	cases := []rustMachineCase{
		{"zero_branch", "branch.rs", 0, 17, true},
		{"division_branch", "branch.rs", 8, 8, true},
		{"arithmetic", "arithmetic.rs", 3, 28, false},
	}
	for _, example := range cases {
		t.Run(example.name, func(t *testing.T) {
			example.verify(t)
		})
	}
}

func (example rustMachineCase) verify(t *testing.T) {
	t.Helper()
	content := compileRustExecutable(t, example.source)
	proof := verifyRustExecutable(t, content, example.input, example.expected)
	if proof.Verification.Status != isla.Proved {
		t.Errorf("correct property: %#v", proof.Verification)
	}
	assertRustExecutionInventory(t, proof, example.hasUnobserved)
	counterexample := verifyRustExecutable(t, content, example.input, example.expected+1)
	if counterexample.Verification.Status != isla.Disproved || counterexample.Verification.CounterexampleState == "" {
		t.Errorf("false property: %#v", counterexample.Verification)
	}
}

func verifyRustExecutable(t *testing.T, content []byte, input uint64, expected uint64) isla.ExecutableResult {
	t.Helper()
	return verifyRustBoundary(t, content, rustProgramBoundary(t, content, input, expected))
}
