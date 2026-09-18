//go:build isla_integration

// Compiler-generated branches keep all static instructions in the result.
// Observed and unobserved instructions must partition the accepted inventory.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func assertRustExecutionInventory(t *testing.T, result isla.ExecutableResult, hasUnobserved bool) {
	t.Helper()
	coverage := result.StaticCoverage
	if !coverage.Complete || coverage.CoveredInstructions != result.Program.InstructionCount {
		t.Errorf("static coverage = %#v", coverage)
	}
	observed := len(result.Execution.Observed)
	unobserved := len(result.Execution.NotObserved)
	if (unobserved > 0) != hasUnobserved || uint64(observed+unobserved) != result.Program.InstructionCount {
		t.Errorf("execution partition = %#v", result.Execution)
	}
	t.Logf("static instructions: %d; observed: %d; not observed: %d", coverage.TotalInstructions, observed, unobserved)
}
