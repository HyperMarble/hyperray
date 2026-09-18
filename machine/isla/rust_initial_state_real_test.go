//go:build isla_integration

// Typed machine state must affect the same public program proof in both stages.
// Fault evidence must come from Sail, not a replacement instruction rule.
package isla_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustTypedTrapState(t *testing.T) {
	useTrapNotificationConfiguration(t)
	assembly := rustPatternCompilerEvidence(t, "function_table.rs")
	assertRustAssemblyFragments(t, assembly, []string{"\taddi\tsp, sp, -32", "\tsd\tra, 24(sp)"})
	content := compileRustExecutable(t, "function_table.rs")
	boundary := rustProgramBoundary(t, content, 0, 0)
	handlerAddress, err := strconv.ParseUint(boundary.InitialRegisters[0].Value, 0, 64)
	if err != nil {
		t.Fatal(err)
	}
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.InitialRegisters = append(boundary.InitialRegisters, isla.RegisterValue{Name: "x2", Value: "0x0"})
	boundary.InitialState = []isla.RegisterValue{
		{Name: "mtvec", Value: fmt.Sprintf("{ bits = 0x%016x }", handlerAddress)},
		{Name: "medeleg", Value: "{ bits = 0x0000000000000000 }"},
	}
	boundary.NegatedAssertion = "~(0:mtval = 0xfffffffffffffff8)"
	proof := verifyRustPatternBoundary(t, content, boundary)
	if proof.Verification.Status != isla.Proved {
		t.Fatalf("fault-address requirement: %s", proof.Verification.Status)
	}
	boundary.NegatedAssertion = "0:mtval = 0xfffffffffffffff8"
	counterexample := verifyRustPatternBoundary(t, content, boundary)
	if counterexample.Verification.Status != isla.Disproved {
		t.Fatalf("changed fault-address requirement: %s", counterexample.Verification.Status)
	}
	if !strings.Contains(counterexample.Verification.CounterexampleState, "0:mtval=#xfffffffffffffff8;") {
		t.Fatalf("missing fault address: %s", counterexample.Verification.CounterexampleState)
	}
	t.Logf("fault-address proof=%s counterexample=%s", proof.Verification.Status, counterexample.Verification.CounterexampleState)
}
