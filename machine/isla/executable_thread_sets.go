// Thread set helpers compare aggregate and per-thread semantic inventories.
// A shared aggregate cannot conceal a missing thread-owned instruction.
package isla

func equalInstructionSets(threads []SemanticThread, aggregate []SemanticInstruction) bool {
	actual := make(map[SemanticInstruction]struct{})
	for _, thread := range threads {
		for _, instruction := range thread.Instructions {
			actual[instruction] = struct{}{}
		}
	}
	expected := make(map[SemanticInstruction]struct{}, len(aggregate))
	for _, instruction := range aggregate {
		expected[instruction] = struct{}{}
	}
	if len(actual) != len(expected) {
		return false
	}
	for instruction := range expected {
		if _, found := actual[instruction]; !found {
			return false
		}
	}
	return true
}

func threadHasAddress(instructions []SemanticInstruction, address uint64) bool {
	for _, instruction := range instructions {
		if instruction.Address == address {
			return true
		}
	}
	return false
}
