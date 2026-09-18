// Proof catalogs bind impossible roots and eliminations to exact artifacts.
// They never accept a proof identifier without a resolved digest.
package coverage

func collectProofs(catalog *catalogs, inventory CompilerInventory) error {
	var err error
	if catalog.impossible, err = impossibleProofIDs(inventory.ImpossiblePreconditionProofs); err != nil {
		return err
	}
	if catalog.proofs, err = eliminationProofIDs(inventory.EliminationProofs); err != nil {
		return err
	}
	impossibleByID := make(map[string]ImpossiblePreconditionProof)
	for _, proof := range inventory.ImpossiblePreconditionProofs {
		impossibleByID[proof.ID] = proof
	}
	for _, id := range sortedIDs(catalog.impossible) {
		if err := requireArtifact(catalog, impossibleByID[id].Artifact, "impossible_proof.artifact"); err != nil {
			return err
		}
	}
	eliminationByID := make(map[string]EliminationProof)
	for _, proof := range inventory.EliminationProofs {
		eliminationByID[proof.ID] = proof
	}
	for _, id := range sortedIDs(catalog.proofs) {
		if err := requireArtifact(catalog, eliminationByID[id].Artifact, "elimination_proof.artifact"); err != nil {
			return err
		}
	}
	return nil
}

func impossibleProofIDs(values []ImpossiblePreconditionProof) (idSet, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return uniqueIDSet(ids, "impossible_precondition_proof")
}

func eliminationProofIDs(values []EliminationProof) (idSet, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return uniqueIDSet(ids, "elimination_proof")
}
