// ARM64 program construction joins the Mach-O image with native Isla input.
// It must not decode instructions or claim native execution support.
package isla

import "github.com/HyperMarble/hyperray/machine/arm64"

// BuildARM64Program loads one bounded Mach-O function and renders its data input.
func BuildARM64Program(content []byte, maximumLoadedBytes uint64, boundary ARM64ProgramBoundary) (Program, error) {
	registers, err := validateARM64Boundary(boundary)
	if err != nil {
		return Program{}, err
	}
	image, err := arm64.LoadFunction(content, maximumLoadedBytes, arm64.FunctionBoundary{
		StartAddress: boundary.FunctionStart, EndAddress: boundary.FunctionEnd,
	})
	if err != nil {
		return Program{}, err
	}
	if err := validateNoLoadTimeRewriting(content, image, boundary); err != nil {
		return Program{}, err
	}
	if err := validateARM64Image(image, boundary); err != nil {
		return Program{}, err
	}
	memory, err := copyValidatedARM64Memory(boundary.Memory, image)
	if err != nil {
		return Program{}, err
	}
	if err := validateARM64Observations(boundary.MemoryObservations, image, memory, registers); err != nil {
		return Program{}, err
	}
	generated, err := renderARM64Program(image, boundary, registers)
	if err != nil {
		return Program{}, err
	}
	program := newProgram(image, generated, reachableInstructions(image.Instructions, boundary.FunctionStart, boundary.FunctionEnd))
	program.profile = image.Profile
	program.functionStart = boundary.FunctionStart
	program.functionEnd = boundary.FunctionEnd
	program.returnAddress = boundary.ReturnAddress
	program.postResetRegisters = copyRegisterValues(registers)
	program.memoryInput = memory
	program.memoryProfile = memoryProfileOf(memory)
	program.memoryIdentity = memoryIdentityOf(memory)
	program.memoryObservations = copyMemoryObservations(boundary.MemoryObservations)
	program.threadEntries = []ThreadEntry{{EntryAddress: boundary.FunctionStart}}
	program.threadIdentity = threadEntryIdentity(program.threadEntries)
	return program, nil
}
