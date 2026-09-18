// ARM64 Program test support uses the checked-in compiler-produced Mach-O.
// It must not rebuild the fixture or infer its boundaries from symbols.
package isla_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

const arm64ProgramReturnAddress = uint64(0x100008000)

func arm64ProgramFixture(t *testing.T) ([]byte, uint64, uint64) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "machine", "arm64", "tiny-arm64-static"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	return content, 0x1000002e8, 0x1000002f0
}

func arm64ProgramBoundary(start uint64, end uint64) isla.ARM64ProgramBoundary {
	return arm64ProgramBoundaryWithReturn(start, end, arm64ProgramReturnAddress)
}

func arm64ProgramBoundaryWithReturn(start uint64, end uint64, returnAddress uint64) isla.ARM64ProgramBoundary {
	return isla.ARM64ProgramBoundary{
		Name: "arm64-public-builder", FunctionStart: start, FunctionEnd: end,
		ReturnAddress: returnAddress,
		PostResetRegisters: []isla.RegisterValue{
			{Name: "R0", Value: "3"}, {Name: "R30", Value: fmt.Sprintf("0x%x", returnAddress)}, {Name: "SP_EL0", Value: "0x3c40"},
		},
		NegatedAssertion:    "~((0:X0 = 4) & (0:SP_EL0 = 0x3c40) & (0:_PC = 0x100008000))",
		MaximumProgramBytes: 1 << 20,
	}
}

func arm64ProgramBoundaryWithProgramCapacity(start uint64, end uint64, capacity uint64) isla.ARM64ProgramBoundary {
	boundary := arm64ProgramBoundary(start, end)
	boundary.MaximumProgramBytes = capacity
	return boundary
}
