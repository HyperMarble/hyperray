// Event extraction accepts instruction records only at trace-event positions.
// Nested value expressions never change the instruction inventory.
package isla

func (inventory *semanticTreeInventory) readEvent(event string, kind string, address semanticTreeAddress, first semanticTreeAddress) (semanticTreeAddress, semanticTreeAddress, error) {
	if kind == "trace" {
		return address, first, semanticProtocolError("trace outside cases")
	}
	parsed, found, err := semanticProgramCounter(event)
	if err != nil {
		return address, first, err
	}
	if found {
		return parsed, first, nil
	}
	if kind != "instr" {
		return address, first, nil
	}
	parts, err := semanticTreeParts(event)
	if err != nil || len(parts) != 2 || !address.found {
		return address, first, semanticProtocolError("instruction has no concrete PC or encoding")
	}
	encoding, err := semanticEncoding(parts[1], "#x", "")
	if err != nil {
		return address, first, err
	}
	instruction := SemanticInstruction{Address: address.value, Encoding: encoding}
	if !first.found {
		first = address
		inventory.entryAddresses[address.value] = struct{}{}
	}
	inventory.instructions[instruction] = struct{}{}
	inventory.encodings[encoding] = struct{}{}
	inventory.count++
	return semanticTreeAddress{}, first, nil
}
