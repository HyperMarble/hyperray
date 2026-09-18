// ARM64 program validation protects the explicit image and reset boundary.
// It must reject aliases, conflicts, and continuations inside loaded memory.
package isla

import (
	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func validateARM64Boundary(boundary ARM64ProgramBoundary) ([]RegisterValue, error) {
	if !plainLine(boundary.Name) || !plainLine(boundary.NegatedAssertion) {
		return nil, engineError(InvalidInput, "ARM64 program text", "name or assertion is not one printable line")
	}
	if boundary.MaximumProgramBytes == 0 || boundary.MaximumProgramBytes > maximumSliceLength() {
		return nil, engineError(InvalidInput, "ARM64 program size", "limit is not finite")
	}
	if boundary.FunctionStart >= boundary.FunctionEnd || boundary.FunctionStart%4 != 0 || boundary.FunctionEnd%4 != 0 {
		return nil, engineError(InvalidInput, "ARM64 function boundary", "must be a nonempty 4-byte aligned interval")
	}
	if boundary.ReturnAddress == 0 || boundary.ReturnAddress%4 != 0 || boundary.ReturnAddress == boundary.FunctionStart {
		return nil, engineError(InvalidInput, "ARM64 return address", "must be nonzero, aligned, and differ from entry")
	}
	return validatedARM64Registers(boundary.PostResetRegisters, boundary.FunctionStart, boundary.ReturnAddress)
}

func validateARM64Image(image machine.Image, boundary ARM64ProgramBoundary) error {
	if image.Profile != arm64.ProfileName || image.EntryAddress != boundary.FunctionStart {
		return engineError(CoverageMismatch, "ARM64 image", "profile or entry differs from boundary")
	}
	if loadedAddress(image, boundary.ReturnAddress) {
		return engineError(InvalidInput, "ARM64 return address", "is inside a loaded image range")
	}
	return nil
}

func loadedAddress(image machine.Image, address uint64) bool {
	for _, loaded := range image.LoadedBytes {
		if loaded.Address == address {
			return true
		}
	}
	return false
}

func maximumSliceLength() uint64 { return uint64(^uint(0) >> 1) }
