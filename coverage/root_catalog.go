// Root-entry catalogs define the exact states or one impossible-case proof.
// They never accept an empty root declaration.
package coverage

func collectRootEntries(catalog *catalogs, inventory CompilerInventory) error {
	ids := make([]string, 0, len(inventory.RootEntries))
	for _, entry := range inventory.RootEntries {
		ids = append(ids, entry.RootID)
		entry.StateIDs = append([]string(nil), entry.StateIDs...)
		entry.ProvenanceEdgeIDs = append([]string(nil), entry.ProvenanceEdgeIDs...)
		catalog.rootEntries[entry.RootID] = entry
	}
	entryIDs, err := uniqueIDSet(ids, "root_entry")
	if err != nil {
		return err
	}
	for _, rootID := range sortedIDs(entryIDs) {
		if err := validateRootEntry(catalog, catalog.rootEntries[rootID]); err != nil {
			return err
		}
	}
	if missing := sortedMissingIDs(catalog.roots, entryIDs); len(missing) != 0 {
		return coverageError("missing_root_entry", missing...)
	}
	return nil
}

func validateRootEntry(catalog *catalogs, entry RootEntry) error {
	if err := requireReference(catalog.roots, entry.RootID, "root"); err != nil {
		return err
	}
	if len(entry.StateIDs) != 0 && entry.ImpossiblePreconditionProofID != "" {
		return coverageError("conflicting_root_entry", entry.RootID)
	}
	if len(entry.StateIDs) != 0 {
		if _, err := checkedReferences(entry.StateIDs, catalog.states, "root_entry.state"); err != nil {
			return err
		}
		return requireExplicitRootPath(catalog, entry)
	}
	if entry.ImpossiblePreconditionProofID == "" {
		return coverageError("missing_root_entry", entry.RootID)
	}
	if err := requireReference(catalog.impossible, entry.ImpossiblePreconditionProofID, "impossible_precondition_proof"); err != nil {
		return err
	}
	catalog.used.impossible[entry.ImpossiblePreconditionProofID] = struct{}{}
	if err := requireImpossibleRootPath(catalog, entry); err != nil {
		return err
	}
	catalog.unsupportedProofs["impossible_precondition:"+entry.RootID] = struct{}{}
	return nil
}
