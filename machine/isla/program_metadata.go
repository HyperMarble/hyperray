// Program metadata exposes immutable architecture and boundary identity.
// It must return copies when the metadata contains caller-owned slices.
package isla

func (program Program) ProfileName() string   { return program.profile }
func (program Program) FunctionStart() uint64 { return program.functionStart }
func (program Program) FunctionEnd() uint64   { return program.functionEnd }
func (program Program) ReturnAddress() uint64 { return program.returnAddress }

func (program Program) MemoryProfile() MemoryProfile { return program.memoryProfile }

func (program Program) MemoryInput() (ARM64MemoryInput, bool) {
	if program.memoryInput == nil {
		return ARM64MemoryInput{}, false
	}
	return copyARM64MemoryInput(*program.memoryInput), true
}

func (program Program) PostResetRegisters() []RegisterValue {
	return copyRegisterValues(program.postResetRegisters)
}

// MemoryObservations returns independent accepted observation metadata.
func (program Program) MemoryObservations() []MemoryObservation {
	return copyMemoryObservations(program.memoryObservations)
}

// Evidence returns the exact executable-to-program identity mapping.
func (program Program) Evidence() ProgramEvidence {
	return ProgramEvidence{
		ProgramDigest: program.digest, ImageDigest: program.imageDigest,
		Profile: program.profile, FunctionStart: program.functionStart,
		FunctionEnd: program.functionEnd, ReturnAddress: program.returnAddress,
		EntryAddress: program.entryAddress, InstructionCount: program.instructionCount,
		LoadedByteCount: program.loadedByteCount, ThreadCount: program.ThreadCount(),
		ThreadEntries: threadEntryIdentity(program.threadEntries), MemoryProfile: program.memoryProfile,
		MemoryIdentity:     program.memoryIdentity,
		MemoryObservations: copyMemoryObservations(program.memoryObservations),
	}
}
