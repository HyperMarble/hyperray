// Footprint requests bind one complete instruction inventory to finite limits.
// They copy caller data before any external process starts.
package isla

import "github.com/HyperMarble/hyperray/machine"

// FootprintRequest contains one bounded instruction-coverage operation.
type FootprintRequest struct {
	release           FootprintRelease
	instructions      []machine.Instruction
	threadLimit       uint64
	timeLimit         uint64
	maximumOutputSize uint64
	pcRegister        string
}

// NewFootprintRequest accepts identified model inputs and a finite inventory.
func NewFootprintRequest(release FootprintRelease, instructions []machine.Instruction, threadLimit uint64, timeLimitSeconds uint64, maximumOutputBytes uint64) (FootprintRequest, error) {
	return newFootprintRequest(release, instructions, threadLimit, timeLimitSeconds, maximumOutputBytes, "PC")
}

func newFootprintRequest(release FootprintRelease, instructions []machine.Instruction, threadLimit uint64, timeLimitSeconds uint64, maximumOutputBytes uint64, pcRegister string) (FootprintRequest, error) {
	if threadLimit == 0 || timeLimitSeconds == 0 || maximumOutputBytes == 0 {
		return FootprintRequest{}, engineError(InvalidInput, "footprint limits", "limits must be more than zero")
	}
	if pcRegister != "PC" && pcRegister != "_PC" {
		return FootprintRequest{}, engineError(UnsupportedProfile, "footprint PC register", pcRegister)
	}
	copied, err := copyInstructions(instructions)
	if err != nil {
		return FootprintRequest{}, err
	}
	request := FootprintRequest{
		release: release, instructions: copied, threadLimit: threadLimit,
		timeLimit: timeLimitSeconds, maximumOutputSize: maximumOutputBytes, pcRegister: pcRegister,
	}
	if err := request.release.current(); err != nil {
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
