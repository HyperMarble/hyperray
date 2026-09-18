//go:build isla_integration

// Symbolic table selection must preserve both compiler-retained call targets.
// A concrete sample cannot replace the complete target set.
package isla_test

import (
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustSymbolicFunctionTable(t *testing.T) {
	assembly := rustPatternCompilerEvidence(t, "function_table.rs")
	assertRustAssemblyFragments(t, assembly, []string{"\tjalr\t", "\tld\t", "\tsd\t"})
	content := compileRustExecutable(t, "function_table.rs")
	boundary := rustProgramBoundary(t, content, 0, 0)
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.InitialRegisters = boundary.InitialRegisters[:1]
	boundary.InitialRegisters = append(boundary.InitialRegisters,
		isla.RegisterValue{Name: "x2", Value: "0x80410000"})
	boundary.NegatedAssertion = "~(0:x10 = 17 | 0:x10 = 64)"
	result := verifyRustPatternBoundary(t, content, boundary)
	if result.Verification.Status != isla.Proved {
		t.Fatalf("impossible table result: status %s", result.Verification.Status)
	}
	assertRustPatternFunctions(t, content, result)
	for _, value := range []uint64{17, 64} {
		boundary.NegatedAssertion = fmt.Sprintf("0:x10 = %d", value)
		counterexample := verifyRustPatternBoundary(t, content, boundary)
		assertSymbolicCounterexample(t, counterexample, value)
	}
}
