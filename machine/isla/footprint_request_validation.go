// Footprint instruction validation rejects malformed or duplicate encodings.
// It runs before any external footprint process starts.
package isla

import (
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

func validateInstruction(instruction machine.Instruction, addresses map[uint64]struct{}) error {
	if instruction.Address%2 != 0 {
		return engineError(InvalidInput, "instruction", fmt.Sprintf("unaligned address %#x", instruction.Address))
	}
	if len(instruction.Bytes) != 2 && len(instruction.Bytes) != 4 {
		return engineError(InvalidInput, "instruction", fmt.Sprintf("address %#x has %d bytes", instruction.Address, len(instruction.Bytes)))
	}
	if _, exists := addresses[instruction.Address]; exists {
		return engineError(InvalidInput, "instruction", fmt.Sprintf("duplicate address %#x", instruction.Address))
	}
	return nil
}
