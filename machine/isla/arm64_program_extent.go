// The instructions a request states, and the ones its calls reach. A program
// built from the whole file asks the engine to consider code the request
// never named; one built from the range alone cannot follow its own calls.
package isla

import (
	"encoding/binary"

	"github.com/HyperMarble/hyperray/machine"
)

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

// reachableInstructions returns the stated range and every instruction its
// calls reach, following each callee to its return.
//
// A function that calls another needs the callee's semantics to execute. The
// whole file is not needed: only what the calls actually reach.
func reachableInstructions(instructions []machine.Instruction, start uint64, end uint64) []machine.Instruction {
	byAddress := make(map[uint64]machine.Instruction, len(instructions))
	order := make([]uint64, 0, len(instructions))
	for _, instruction := range instructions {
		byAddress[instruction.Address] = instruction
		order = append(order, instruction.Address)
	}
	wanted := make(map[uint64]bool)
	for address := start; address < end; address += 4 {
		wanted[address] = true
	}
	pending := callTargets(instructionsUnderProof(instructions, start, end))
	for len(pending) > 0 {
		target := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, address := range calleeExtent(byAddress, target) {
			if wanted[address] {
				continue
			}
			wanted[address] = true
			pending = append(pending, callTargets([]machine.Instruction{byAddress[address]})...)
		}
	}
	reached := make([]machine.Instruction, 0, len(wanted))
	for _, address := range order {
		if wanted[address] {
			reached = append(reached, byAddress[address])
		}
	}
	return reached
}

// calleeExtent returns a called function's instructions, up to its return.
//
// A callee with no return instruction within the image yields nothing, so a
// missing function is visible rather than silently partial.
func calleeExtent(byAddress map[uint64]machine.Instruction, entry uint64) []uint64 {
	const returnLimit = 4096
	extent := make([]uint64, 0, 16)
	for address := entry; address < entry+returnLimit*4; address += 4 {
		instruction, present := byAddress[address]
		if !present {
			return nil
		}
		extent = append(extent, address)
		if isReturn(instruction) {
			return extent
		}
	}
	return nil
}

// callTargets returns every address these instructions call.
func callTargets(instructions []machine.Instruction) []uint64 {
	targets := make([]uint64, 0, len(instructions))
	for _, instruction := range instructions {
		if target, calls := callTarget(instruction); calls {
			targets = append(targets, target)
		}
	}
	return targets
}

// callTarget reports the address a BL instruction calls.
func callTarget(instruction machine.Instruction) (uint64, bool) {
	if len(instruction.Bytes) != 4 {
		return 0, false
	}
	word := binary.LittleEndian.Uint32(instruction.Bytes)
	// A64 BL is 100101 followed by a signed 26-bit word offset.
	if word>>26 != 0x25 {
		return 0, false
	}
	offset := int64(int32(word<<6)>>6) * 4
	return uint64(int64(instruction.Address) + offset), true
}

// isReturn reports whether the instruction is A64 RET.
func isReturn(instruction machine.Instruction) bool {
	if len(instruction.Bytes) != 4 {
		return false
	}
	return binary.LittleEndian.Uint32(instruction.Bytes)&0xfffffc1f == 0xd65f0000
}
