// Binding reference checks resolve all forward mapping identifiers.
// They never accept a mapping for the wrong operation kind.
package coverage

func requireMappedOperation(catalog *catalogs, id string, kind OperationKind) error {
	operation, exists := catalog.operations[id]
	if !exists {
		return coverageError("unknown_reference", "operation", id)
	}
	if operation.Kind != kind {
		return coverageError("wrong_mapping_kind", id, string(kind))
	}
	if operation.Disposition != DispositionMapped {
		return coverageError("wrong_operation_disposition", id, string(operation.Disposition))
	}
	return nil
}

func requireMachineReferences(catalog *catalogs, binding MachineBinding) error {
	err := requireEvidence(
		evidenceField{"machine.operation_id", binding.OperationID},
		evidenceField{"machine.compiler_output_id", binding.CompilerOutputID},
		evidenceField{"machine.instruction_id", binding.InstructionID},
		evidenceField{"machine.semantic_rule_id", binding.SemanticRuleID},
		evidenceField{"machine.transition_id", binding.TransitionID},
		evidenceField{"machine.operation_to_output", binding.OperationToCompilerOutputEdgeID},
		evidenceField{"machine.output_to_instruction", binding.CompilerOutputToInstructionEdgeID},
		evidenceField{"machine.instruction_to_rule", binding.InstructionToSemanticRuleEdgeID},
		evidenceField{"machine.rule_to_transition", binding.SemanticRuleToTransitionEdgeID},
	)
	if err != nil {
		return err
	}
	if err := requireMappedOperation(catalog, binding.OperationID, OperationCompiler); err != nil {
		return err
	}
	references := []struct {
		ids   idSet
		id    string
		label string
	}{
		{catalog.outputs, binding.CompilerOutputID, "compiler_output"},
		{catalog.instructions, binding.InstructionID, "image_instruction"},
		{catalog.rules, binding.SemanticRuleID, "semantic_rule"},
		{catalog.transitions, binding.TransitionID, "transition"},
	}
	for _, reference := range references {
		if err := requireReference(reference.ids, reference.id, reference.label); err != nil {
			return err
		}
	}
	return requireMachinePath(catalog, binding)
}
