// Execution inventories distinguish observed instructions from the static set.
// NotObserved is not an independent proof of unreachability.
package isla

// ExecutionInventory partitions the static inventory for one accepted query.
type ExecutionInventory struct {
	Observed    []SemanticInstruction `json:"observed"`
	NotObserved []SemanticInstruction `json:"not_observed"`
	Threads     []SemanticThread      `json:"threads"`
}

func programExecutionInventory(program Program, report SemanticReport) ExecutionInventory {
	remaining := expectedSemanticInstructions(program.instructions)
	observed := make(map[SemanticInstruction]struct{}, len(report.Instructions))
	for _, instruction := range report.Instructions {
		observed[instruction] = struct{}{}
		delete(remaining, instruction)
	}
	return ExecutionInventory{
		Observed: sortedSemanticInstructions(observed), NotObserved: sortedSemanticInstructions(remaining),
		Threads: copySemanticThreads(report.Threads),
	}
}
