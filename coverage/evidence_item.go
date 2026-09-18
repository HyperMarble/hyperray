// Evidence-item catalogs name compiler outputs, instructions, and rules.
// They never borrow an artifact reference from a mapping claim.
package coverage

func collectEvidenceItems(catalog *catalogs, inventory CompilerInventory) error {
	var err error
	if catalog.outputs, err = outputIDs(inventory.CompilerOutputs); err != nil {
		return err
	}
	if catalog.instructions, err = instructionIDs(inventory.ImageInstructions); err != nil {
		return err
	}
	if catalog.rules, err = ruleIDs(inventory.SemanticRules); err != nil {
		return err
	}
	if err := validateOutputs(catalog, inventory.CompilerOutputs); err != nil {
		return err
	}
	if err := validateInstructions(catalog, inventory.ImageInstructions); err != nil {
		return err
	}
	return validateRules(catalog, inventory.SemanticRules)
}

func outputIDs(values []CompilerOutput) (idSet, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return uniqueIDSet(ids, "compiler_output")
}

func instructionIDs(values []ImageInstruction) (idSet, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return uniqueIDSet(ids, "image_instruction")
}

func ruleIDs(values []SemanticRule) (idSet, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return uniqueIDSet(ids, "semantic_rule")
}
