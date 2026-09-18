// Machine bindings declare compiler ownership and mark reverse-catalog use.
// They never accept two owners for one transition.
package coverage

func collectMachineBindings(catalog *catalogs, values []MachineBinding) error {
	ordered := orderedMachineBindings(values)
	for index, binding := range ordered {
		if index != 0 && binding == ordered[index-1] {
			return coverageError("duplicate_binding", binding.OperationID, binding.TransitionID)
		}
		if err := requireMachineReferences(catalog, binding); err != nil {
			return err
		}
		owner := "compiler:" + binding.InstructionID + ":" + binding.SemanticRuleID
		if err := claimTransition(catalog, binding.TransitionID, owner); err != nil {
			return err
		}
		catalog.machine[binding] = struct{}{}
		catalog.used.outputs[binding.CompilerOutputID] = struct{}{}
		catalog.used.instructions[binding.InstructionID] = struct{}{}
		catalog.used.rules[binding.SemanticRuleID] = struct{}{}
		catalog.used.operations[binding.OperationID] = struct{}{}
	}
	return nil
}
