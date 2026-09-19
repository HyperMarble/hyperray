// Splitting an inventory into the traces already stored and the ones missing.
// The caller's order is kept, so a trace never moves to another instruction.
package isla

import "github.com/HyperMarble/hyperray/machine"

// storedTraces answers each instruction from the store where it can.
//
// A stored entry takes the caller's address. An instruction with no entry
// leaves a hole, reported as missing for tracing.
func storedTraces(store *TraceStore, architecture string,
	instructions []machine.Instruction) ([]InstructionTrace, []machine.Instruction) {
	traces := make([]InstructionTrace, len(instructions))
	missing := make([]machine.Instruction, 0, len(instructions))
	for index := range instructions {
		instruction := instructions[index]
		stored, found := store.Lookup(encodingOf(instruction), architecture)
		if !found {
			missing = append(missing, instruction)
			continue
		}
		stored.Address = instruction.Address
		traces[index] = stored
	}
	return traces, missing
}

// placeTraced fills each hole with the trace made for it, in the same order.
func placeTraced(traces []InstructionTrace, traced []InstructionTrace) []InstructionTrace {
	next := 0
	for index := range traces {
		if traces[index].Encoding != "" {
			continue
		}
		if next >= len(traced) {
			return traces
		}
		traces[index] = traced[next]
		next++
	}
	return traces
}
