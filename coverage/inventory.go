// Core inventory validation fixes function, root, and operation identities.
// It never accepts an empty proof boundary.
package coverage

func collectCore(catalog *catalogs, inventory CompilerInventory) error {
	if len(inventory.Functions) == 0 || len(inventory.Roots) == 0 {
		return coverageError("empty_inventory", "functions", "roots")
	}
	if len(inventory.Operations) == 0 {
		return coverageError("empty_inventory", "operations")
	}
	functionIDs := make([]string, 0, len(inventory.Functions))
	rootIDs := make([]string, 0, len(inventory.Roots))
	operationIDs := make([]string, 0, len(inventory.Operations))
	for _, function := range inventory.Functions {
		functionIDs = append(functionIDs, function.ID)
	}
	for _, root := range inventory.Roots {
		rootIDs = append(rootIDs, root.ID)
	}
	for _, operation := range inventory.Operations {
		operationIDs = append(operationIDs, operation.ID)
		catalog.operations[operation.ID] = operation
	}
	var err error
	if catalog.functions, err = uniqueIDSet(functionIDs, "function"); err != nil {
		return err
	}
	if catalog.roots, err = uniqueIDSet(rootIDs, "root"); err != nil {
		return err
	}
	if _, err = uniqueIDSet(operationIDs, "operation"); err != nil {
		return err
	}
	return validateCore(catalog, inventory)
}
