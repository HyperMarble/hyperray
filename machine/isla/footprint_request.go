// Footprint requests bind one complete instruction inventory to finite limits.
// They copy caller data before any external process starts.
package isla

import (
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

// FootprintRequest contains one bounded instruction-coverage operation.
type FootprintRequest struct {
	architecture      Artifact
	configuration     Artifact
	instructions      []machine.Instruction
	threadLimit       uint64
	timeLimit         uint64
	maximumOutputSize uint64
}

// NewFootprintRequest accepts identified model inputs and a finite inventory.
func NewFootprintRequest(architecture Artifact, configuration Artifact, instructions []machine.Instruction, threadLimit uint64, timeLimitSeconds uint64, maximumOutputBytes uint64) (FootprintRequest, error) {
	if threadLimit == 0 || timeLimitSeconds == 0 || maximumOutputBytes == 0 {
		return FootprintRequest{}, engineError(InvalidInput, "footprint limits", "limits must be more than zero")
	}
	copied, err := copyInstructions(instructions)
	if err != nil {
		return FootprintRequest{}, err
	}
	request := FootprintRequest{
		architecture: architecture, configuration: configuration,
		instructions: copied, threadLimit: threadLimit,
		timeLimit: timeLimitSeconds, maximumOutputSize: maximumOutputBytes,
	}
	if err := request.current(); err != nil {
		return FootprintRequest{}, err
	}
	return request, nil
}

func copyInstructions(values []machine.Instruction) ([]machine.Instruction, error) {
	if len(values) == 0 {
		return nil, engineError(InvalidInput, "instruction inventory", "empty")
	}
	result := make([]machine.Instruction, 0, len(values))
	addresses := make(map[uint64]struct{}, len(values))
	for index := range values {
		instruction := values[index]
		if err := validateInstruction(instruction, addresses); err != nil {
			return nil, err
		}
		instruction.Bytes = append([]byte(nil), instruction.Bytes...)
		result = append(result, instruction)
		addresses[instruction.Address] = struct{}{}
	}
	return result, nil
}

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
