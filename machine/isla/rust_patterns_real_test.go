//go:build isla_integration

// Compiler-generated calls and storage use the unchanged public proof API.
// A failing construct must remain visible rather than become an exclusion.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustPatterns(t *testing.T) {
	cases := []rustMachineCase{
		{name: "generic_calls", source: "generic_calls.rs", input: 5, expected: 20},
		{name: "dynamic_call", source: "dynamic_call.rs", input: 5, expected: 12},
		{name: "recursive_calls", source: "recursive_calls.rs", input: 3, expected: 6},
		{name: "stack_array", source: "stack_array.rs", input: 2, expected: 13},
		{name: "stack_values", source: "stack_values.rs", input: 2, expected: 13},
	}
	for _, example := range cases {
		t.Run(example.name, func(t *testing.T) {
			verifyRustPattern(t, example)
		})
	}
}

func verifyRustPattern(t *testing.T, example rustMachineCase) {
	t.Helper()
	content := compileRustExecutable(t, example.source)
	boundary := rustProgramBoundary(t, content, example.input, example.expected)
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.InitialRegisters = append(boundary.InitialRegisters,
		isla.RegisterValue{Name: "x2", Value: "0x80410000"})
	result := verifyRustPatternBoundary(t, content, boundary)
	if result.Verification.Status != isla.Proved {
		t.Fatalf("declared result: status %s", result.Verification.Status)
	}
	assertRustPatternFunctions(t, content, result)
	t.Logf("static instructions: %d; observed: %d; not observed: %d",
		result.StaticCoverage.TotalInstructions, len(result.Execution.Observed), len(result.Execution.NotObserved))
	wrong := rustProgramBoundary(t, content, example.input, example.expected+1)
	wrong.MemoryProfile = boundary.MemoryProfile
	wrong.InitialRegisters = boundary.InitialRegisters
	counterexample := verifyRustPatternBoundary(t, content, wrong)
	assertSymbolicCounterexample(t, counterexample, example.expected)
}
