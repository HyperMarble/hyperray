// Binding reconciliation gives every transition one declared semantic owner.
// It never treats an elimination equivalence as ownership.
package coverage

import "sort"

func collectBindings(catalog *catalogs, inventory CompilerInventory) error {
	if err := validateProvenanceNodes(catalog); err != nil {
		return err
	}
	if err := collectMachineBindings(catalog, inventory.MachineBindings); err != nil {
		return err
	}
	if err := collectSemanticBindings(catalog, inventory.SemanticBindings); err != nil {
		return err
	}
	owned := make(idSet, len(catalog.transitionOwners))
	for transitionID := range catalog.transitionOwners {
		owned[transitionID] = struct{}{}
	}
	if missing := sortedMissingIDs(catalog.transitions, owned); len(missing) != 0 {
		return coverageError("missing_transition_binding", missing...)
	}
	return nil
}

func claimTransition(catalog *catalogs, transitionID string, owner string) error {
	previous, exists := catalog.transitionOwners[transitionID]
	if !exists {
		catalog.transitionOwners[transitionID] = owner
		return nil
	}
	if previous == owner {
		return nil
	}
	owners := []string{previous, owner}
	sort.Strings(owners)
	return coverageError("conflicting_transition_binding", transitionID, owners[0], owners[1])
}
