// Evidence-item artifact checks run in stable catalog identifier order.
// They never permit a stale artifact digest.
package coverage

func validateOutputs(catalog *catalogs, values []CompilerOutput) error {
	byID := make(map[string]CompilerOutput, len(values))
	for _, value := range values {
		byID[value.ID] = value
	}
	for _, id := range sortedIDs(catalog.outputs) {
		if err := requireArtifact(catalog, byID[id].Artifact, "compiler_output.artifact"); err != nil {
			return err
		}
	}
	return nil
}

func validateInstructions(catalog *catalogs, values []ImageInstruction) error {
	byID := make(map[string]ImageInstruction, len(values))
	for _, value := range values {
		byID[value.ID] = value
	}
	for _, id := range sortedIDs(catalog.instructions) {
		if err := requireArtifact(catalog, byID[id].Artifact, "image_instruction.artifact"); err != nil {
			return err
		}
	}
	return nil
}

func validateRules(catalog *catalogs, values []SemanticRule) error {
	byID := make(map[string]SemanticRule, len(values))
	for _, value := range values {
		byID[value.ID] = value
	}
	for _, id := range sortedIDs(catalog.rules) {
		if err := requireArtifact(catalog, byID[id].Artifact, "semantic_rule.artifact"); err != nil {
			return err
		}
	}
	return nil
}
