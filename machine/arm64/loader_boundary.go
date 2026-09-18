// This file validates the explicit ARM64 analysis boundary.
// It must not infer a boundary from symbols or thread metadata.
package arm64

import (
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

func validateBoundary(boundary FunctionBoundary, regions []machine.ExecutableRegion) error {
	if boundary.StartAddress >= boundary.EndAddress || boundary.StartAddress%4 != 0 || boundary.EndAddress%4 != 0 {
		return &Rejection{Code: InvalidFunctionBoundary, Detail: fmt.Sprintf("range %#x:%#x is empty or not 4-byte aligned", boundary.StartAddress, boundary.EndAddress)}
	}
	for _, region := range regions {
		if boundary.StartAddress < region.StartAddress {
			continue
		}
		end := region.StartAddress + region.ByteLength
		if boundary.EndAddress <= end {
			return nil
		}
	}
	return &Rejection{Code: InvalidFunctionBoundary, Detail: fmt.Sprintf("range %#x:%#x is outside pure code", boundary.StartAddress, boundary.EndAddress)}
}
