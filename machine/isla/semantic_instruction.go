// Semantic instruction parsing binds executed encodings to branch-local PCs.
// Symbolic addresses on executed instructions stop coverage acceptance.
package isla

import (
	"strconv"
	"strings"
)

// SemanticInstruction identifies one executed instruction location and value.
type SemanticInstruction struct {
	Address  uint64 `json:"address"`
	Encoding string `json:"encoding"`
}

func semanticInstructionEvents(tree string) (map[SemanticInstruction]struct{}, map[string]struct{}, uint64, error) {
	result, err := semanticInstructionInventory(tree)
	if err != nil {
		return nil, nil, 0, err
	}
	return result.instructions, result.encodings, result.count, nil
}

func semanticInstructionInventory(tree string) (semanticTreeInventory, error) {
	result := semanticTreeInventory{
		instructions:   make(map[SemanticInstruction]struct{}),
		encodings:      make(map[string]struct{}),
		entryAddresses: make(map[uint64]struct{}),
	}
	if err := result.readTrace(tree, semanticTreeAddress{}); err != nil {
		return semanticTreeInventory{}, err
	}
	return result, nil
}

func semanticProgramCounter(value string) (semanticTreeAddress, bool, error) {
	parts, err := semanticTreeParts(value)
	if err != nil {
		return semanticTreeAddress{}, false, err
	}
	if parts[0] != "read-reg" || len(parts) < 2 || (parts[1] != "|PC|" && parts[1] != "|_PC|") {
		return semanticTreeAddress{}, false, nil
	}
	if len(parts) != 4 || parts[2] != "nil" {
		return semanticTreeAddress{}, false, semanticProtocolError("invalid instruction PC event")
	}
	if strings.HasPrefix(parts[3], "v") {
		symbol, err := strconv.ParseUint(strings.TrimPrefix(parts[3], "v"), 10, 32)
		if err != nil || "v"+strconv.FormatUint(symbol, 10) != parts[3] {
			return semanticTreeAddress{}, false, semanticProtocolError("invalid instruction PC symbol")
		}
		return semanticTreeAddress{}, true, nil
	}
	if !strings.HasPrefix(parts[3], "#x") {
		return semanticTreeAddress{}, false, semanticProtocolError("instruction PC is not concrete")
	}
	address, err := strconv.ParseUint(strings.TrimPrefix(parts[3], "#x"), 16, 64)
	if err != nil {
		return semanticTreeAddress{}, false, semanticProtocolError("invalid instruction PC")
	}
	return semanticTreeAddress{value: address, found: true}, true, nil
}
