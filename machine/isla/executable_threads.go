// Per-thread semantic inventories retain ownership before executable joining.
// Aggregate sets cannot satisfy another thread's declared entry.
package isla

func matchSemanticThreads(
	declared []ThreadEntry,
	reported []SemanticThread,
	aggregate []SemanticInstruction,
	expected map[SemanticInstruction]struct{},
) error {
	if len(reported) == 0 {
		return engineError(CoverageMismatch, "executed instruction inventory", "missing per-thread records")
	}
	if len(reported) != len(declared) {
		return engineError(CoverageMismatch, "executed instruction inventory", "per-thread record count differs")
	}
	for index := range reported {
		if err := matchSemanticThread(declared[index], reported[index], uint64(index), expected); err != nil {
			return err
		}
	}
	if !equalInstructionSets(reported, aggregate) {
		return engineError(CoverageMismatch, "executed instruction inventory", "aggregate and per-thread sets differ")
	}
	return nil
}

func matchSemanticThread(declared ThreadEntry, reported SemanticThread, index uint64, expected map[SemanticInstruction]struct{}) error {
	if reported.ID != index || reported.EntryAddress != declared.EntryAddress {
		return engineError(CoverageMismatch, "executed instruction inventory", "thread entry differs")
	}
	if len(reported.EntryAddresses) != 1 || reported.EntryAddresses[0] != declared.EntryAddress {
		return engineError(CoverageMismatch, "executed instruction inventory", "thread has multiple or unexpected first PCs")
	}
	if !threadHasAddress(reported.Instructions, reported.EntryAddress) {
		return engineError(CoverageMismatch, "executed instruction inventory", "thread entry event missing")
	}
	seen := make(map[SemanticInstruction]struct{}, len(reported.Instructions))
	for _, instruction := range reported.Instructions {
		if _, found := seen[instruction]; found {
			return semanticInventoryError("duplicate", instruction)
		}
		seen[instruction] = struct{}{}
		if _, found := expected[instruction]; !found {
			return semanticInventoryError("extra", instruction)
		}
	}
	return nil
}
