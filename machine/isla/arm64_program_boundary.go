// ARM64 program boundaries bind a checked image region to reset-time inputs.
// They must not infer function extents or execution results.
package isla

// ARM64ProgramBoundary declares one static ARM64 function analysis region.
type ARM64ProgramBoundary struct {
	Name                string
	FunctionStart       uint64
	FunctionEnd         uint64
	ReturnAddress       uint64
	PostResetRegisters  []RegisterValue
	NegatedAssertion    string
	MaximumProgramBytes uint64
	Memory              *ARM64MemoryInput
	MemoryObservations  []MemoryObservation
	// NativeRegisterNames are the registers the model in use declares, so an
	// observation cannot take one of their names. ModelRegisterNames reads
	// them from the model file; a boundary with none reserves only the
	// fixed A64 aliases.
	NativeRegisterNames []string
}
