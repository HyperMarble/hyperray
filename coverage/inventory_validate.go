// Core reference checks enforce rooted functions and valid operation modes.
// They never interpret a missing operation as compiler elimination.
package coverage

func validateCore(catalog *catalogs, inventory CompilerInventory) error {
	rootByID := make(map[string]Root, len(inventory.Roots))
	rootedFunctions := make(idSet)
	for _, root := range inventory.Roots {
		rootByID[root.ID] = root
	}
	for _, rootID := range sortedIDs(catalog.roots) {
		root := rootByID[rootID]
		if err := requireReference(catalog.functions, root.FunctionID, "function"); err != nil {
			return err
		}
		rootedFunctions[root.FunctionID] = struct{}{}
	}
	if missing := sortedMissingIDs(catalog.functions, rootedFunctions); len(missing) != 0 {
		return coverageError("missing_root", missing...)
	}
	for _, operationID := range sortedOperationIDs(catalog.operations) {
		if err := validateOperation(catalog, catalog.operations[operationID]); err != nil {
			return err
		}
	}
	return nil
}

func validateOperation(catalog *catalogs, operation Operation) error {
	if err := requireEvidence(evidenceField{"operation.location", operation.Location}); err != nil {
		return err
	}
	if operation.FunctionID != "" {
		if err := requireReference(catalog.functions, operation.FunctionID, "function"); err != nil {
			return err
		}
	}
	if err := validateOperationKind(operation); err != nil {
		return err
	}
	return validateDisposition(operation)
}
