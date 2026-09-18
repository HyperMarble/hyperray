// Program boundaries declare machine inputs and finite safety requirements.
// Model-call names do not define replacement instruction behavior.
package isla

// RegisterValue gives one concrete initial machine-register value.
type RegisterValue struct {
	Name  string
	Value string
}

// ThreadEntry declares one initial machine thread in ordered program input.
type ThreadEntry struct {
	EntryAddress     uint64
	InitialRegisters []RegisterValue
}

// ProgramBoundary defines one finite executable proof query.
type ProgramBoundary struct {
	Name                string
	ThreadAddress       uint64
	InitialRegisters    []RegisterValue
	Threads             []ThreadEntry
	InitialState        []RegisterValue
	ForbiddenModelCalls []string
	NegatedAssertion    string
	MaximumProgramBytes uint64
	MemoryProfile       MemoryProfile
}
