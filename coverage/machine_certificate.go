// Machine certificate checks require an exact independent binding for each row.
// They never accept an extra or omitted machine claim.
package coverage

func checkMachineMappings(catalog catalogs, mappings []MachineMapping) error {
	values := make([]MachineBinding, 0, len(mappings))
	for _, mapping := range mappings {
		values = append(values, MachineBinding(mapping))
	}
	ordered := orderedMachineBindings(values)
	seen := make(map[MachineBinding]struct{}, len(ordered))
	for index, binding := range ordered {
		if index != 0 && binding == ordered[index-1] {
			return coverageError("duplicate_evidence", binding.OperationID, binding.TransitionID)
		}
		if err := requireMachineReferences(&catalog, binding); err != nil {
			return err
		}
		if _, exists := catalog.machine[binding]; !exists {
			return coverageError("machine_mapping_mismatch", binding.OperationID, binding.TransitionID)
		}
		seen[binding] = struct{}{}
	}
	missing := make([]string, 0)
	declared := make([]MachineBinding, 0, len(catalog.machine))
	for binding := range catalog.machine {
		declared = append(declared, binding)
	}
	for _, binding := range orderedMachineBindings(declared) {
		if _, exists := seen[binding]; !exists {
			missing = append(missing, binding.OperationID+":"+binding.TransitionID)
		}
	}
	if len(missing) != 0 {
		return coverageError("missing_machine_mapping", missing...)
	}
	return nil
}
