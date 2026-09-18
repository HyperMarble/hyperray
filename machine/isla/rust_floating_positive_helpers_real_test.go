//go:build isla_integration

package isla_test

import (
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func floatingBoundary(t *testing.T, content []byte, left, right, flags, result, resultFlags uint64) isla.ProgramBoundary {
	t.Helper()
	boundary := rustProgramBoundary(t, content, 0, 0)
	boundary.InitialRegisters = append(boundary.InitialRegisters[:1],
		isla.RegisterValue{Name: "f10", Value: fmt.Sprintf("0x%016x", left)},
		isla.RegisterValue{Name: "f11", Value: fmt.Sprintf("0x%016x", right)})
	boundary.InitialState = []isla.RegisterValue{
		{Name: "misa", Value: "{ bits = 0x000000000000112d }"},
		{Name: "mstatus", Value: fmt.Sprintf("{ bits = 0x%016x }", floatingCleanMstatus())},
		{Name: "fcsr", Value: fmt.Sprintf("{ bits = 0x%08x }", flags)},
	}
	boundary.NegatedAssertion = floatingAssertion(result, resultFlags, floatingDirtyMstatus())
	return boundary
}

func floatingAssertion(result, flags, mstatus uint64) string {
	return fmt.Sprintf("~(0:f10 = 0x%016x & 0:fcsr.bits = 0x%08x & 0:mstatus.bits = 0x%016x)", result, flags, mstatus)
}

func floatingCleanMstatus() uint64 {
	// The pinned model maps Clean to 0b10 at mstatus[14..13].
	return 2 << 13
}

func floatingDirtyMstatus() uint64 {
	// dirty_fd_context maps Dirty to 0b11 and sets SD at bit 63.
	return (3 << 13) | (uint64(1) << 63)
}
