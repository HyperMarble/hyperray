// Thread entries bind each declared start to the loaded instruction inventory.
// They must not infer validity from a segment address range.
package isla

import "github.com/HyperMarble/hyperray/machine"

func validatedThreads(image machine.Image, boundary ProgramBoundary) ([]ThreadEntry, error) {
	if boundary.Threads == nil {
		if boundary.ThreadAddress != image.EntryAddress {
			return nil, engineError(CoverageMismatch, "thread address", "does not equal the ELF entry address")
		}
		if !instructionStart(image, boundary.ThreadAddress) {
			return nil, engineError(CoverageMismatch, "thread entry", "is not an instruction start")
		}
		registers, err := validatedRegisters(boundary.InitialRegisters)
		if err != nil {
			return nil, err
		}
		return []ThreadEntry{{EntryAddress: boundary.ThreadAddress, InitialRegisters: registers}}, nil
	}
	if boundary.ThreadAddress != 0 || len(boundary.InitialRegisters) != 0 {
		return nil, engineError(InvalidInput, "thread boundary", "legacy entry and registers conflict with explicit threads")
	}
	if len(boundary.Threads) == 0 {
		return nil, engineError(InvalidInput, "threads", "explicit set is empty")
	}
	if boundary.MemoryProfile == SequentialMemory && len(boundary.Threads) > 1 {
		return nil, engineError(InvalidInput, "threads", "sequential memory supports one thread")
	}
	result := make([]ThreadEntry, len(boundary.Threads))
	for index := range boundary.Threads {
		thread := boundary.Threads[index]
		if !instructionStart(image, thread.EntryAddress) {
			return nil, engineError(CoverageMismatch, "thread entry", "is not an instruction start")
		}
		registers, err := validatedRegisters(thread.InitialRegisters)
		if err != nil {
			return nil, err
		}
		result[index] = ThreadEntry{EntryAddress: thread.EntryAddress, InitialRegisters: registers}
	}
	return result, nil
}

func instructionStart(image machine.Image, address uint64) bool {
	for index := range image.Instructions {
		if image.Instructions[index].Address == address {
			return true
		}
	}
	return false
}
