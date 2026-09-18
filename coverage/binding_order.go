// Binding ordering gives catalog and certificate rows one stable order.
// It never uses input order to select an error.
package coverage

import (
	"cmp"
	"sort"
)

func orderedMachineBindings(values []MachineBinding) []MachineBinding {
	ordered := append([]MachineBinding(nil), values...)
	sort.Slice(ordered, func(left int, right int) bool {
		first := ordered[left]
		second := ordered[right]
		return cmp.Or(
			cmp.Compare(first.OperationID, second.OperationID),
			cmp.Compare(first.CompilerOutputID, second.CompilerOutputID),
			cmp.Compare(first.InstructionID, second.InstructionID),
			cmp.Compare(first.SemanticRuleID, second.SemanticRuleID),
			cmp.Compare(first.TransitionID, second.TransitionID),
			cmp.Compare(first.OperationToCompilerOutputEdgeID, second.OperationToCompilerOutputEdgeID),
			cmp.Compare(first.CompilerOutputToInstructionEdgeID, second.CompilerOutputToInstructionEdgeID),
			cmp.Compare(first.InstructionToSemanticRuleEdgeID, second.InstructionToSemanticRuleEdgeID),
			cmp.Compare(first.SemanticRuleToTransitionEdgeID, second.SemanticRuleToTransitionEdgeID),
		) < 0
	})
	return ordered
}

func orderedSemanticBindings(values []SemanticBinding) []SemanticBinding {
	ordered := append([]SemanticBinding(nil), values...)
	sort.Slice(ordered, func(left int, right int) bool {
		first := ordered[left]
		second := ordered[right]
		return cmp.Or(
			cmp.Compare(first.OperationID, second.OperationID),
			cmp.Compare(first.Kind, second.Kind),
			cmp.Compare(first.SemanticRuleID, second.SemanticRuleID),
			cmp.Compare(first.TransitionID, second.TransitionID),
			cmp.Compare(first.OperationToSemanticRuleEdgeID, second.OperationToSemanticRuleEdgeID),
			cmp.Compare(first.SemanticRuleToTransitionEdgeID, second.SemanticRuleToTransitionEdgeID),
		) < 0
	})
	return ordered
}
