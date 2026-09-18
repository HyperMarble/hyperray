// The instructions a request states, not every instruction in the file. A
// program built from the whole image asks the engine to consider code the
// request never named.
package isla

import "github.com/HyperMarble/hyperray/machine"

// instructionsUnderProof returns the instructions inside the stated
// boundary, in the order the image reports them.
//
// An empty result is returned as such: the boundary is checked elsewhere,
// and silently proving nothing would be worse than an empty program.
func instructionsUnderProof(instructions []machine.Instruction, start uint64, end uint64) []machine.Instruction {
	within := make([]machine.Instruction, 0, len(instructions))
	for _, instruction := range instructions {
		if instruction.Address < start || instruction.Address >= end {
			continue
		}
		within = append(within, instruction)
	}
	return within
}
