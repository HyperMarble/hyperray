// Semantic tree records retain per-thread instruction and entry inventories.
// Aggregate instruction sets must not erase thread ownership.
package isla

import "sort"

func (threads *semanticThreads) addTree(tree string) error {
	inventory, err := semanticInstructionInventory(tree)
	if err != nil {
		return err
	}
	threads.traceCount++
	threads.eventCount += inventory.count
	mergeEncodingSet(threads.encodings, inventory.encodings)
	mergeInstructionSet(threads.instructions, inventory.instructions)
	addresses := make([]uint64, 0, len(inventory.entryAddresses))
	for address := range inventory.entryAddresses {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool { return addresses[i] < addresses[j] })
	entry := uint64(0)
	if len(addresses) == 1 {
		entry = addresses[0]
	}
	threads.records = append(threads.records, SemanticThread{
		ID: uint64(len(threads.records)), EntryAddress: entry, EntryAddresses: addresses,
		InstructionEventCount: inventory.count, Instructions: sortedSemanticInstructions(inventory.instructions),
	})
	return nil
}
