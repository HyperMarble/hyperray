//go:build isla_integration

// Fault fixtures declare a bounded trap-entry address from the compiled ELF.
// This boundary does not represent an operating-system handler.
package isla_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func rustFaultSafetyBoundary(t *testing.T, content []byte, stack string) isla.ProgramBoundary {
	t.Helper()
	boundary := rustProgramBoundary(t, content, 0, 17)
	handler, err := strconv.ParseUint(boundary.InitialRegisters[0].Value, 0, 64)
	if err != nil {
		t.Fatal(err)
	}
	boundary.MemoryProfile = isla.SequentialMemory
	boundary.InitialRegisters = append(boundary.InitialRegisters, isla.RegisterValue{Name: "x2", Value: stack})
	boundary.InitialState = []isla.RegisterValue{
		{Name: "mtvec", Value: fmt.Sprintf("{ bits = 0x%016x }", handler)},
		{Name: "medeleg", Value: "{ bits = 0x0000000000000000 }"},
	}
	boundary.ForbiddenModelCalls = []string{"trap_handler"}
	return boundary
}
