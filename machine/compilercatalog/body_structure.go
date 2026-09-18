// Structural payload validation compares every retained compiler position.
// Nested compiler values remain opaque and are never treated as proof.
package compilercatalog

import (
	"encoding/json"
	"fmt"
)

type structuralBlock struct {
	Index      int               `json:"index"`
	Statements []json.RawMessage `json:"statements"`
	Terminator json.RawMessage   `json:"terminator"`
}

func validateStructuralPositions(instance Instance) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(instance.Body.Payload, &payload); err != nil {
		return fmt.Errorf("decode body payload for %s: %w", instance.ID, err)
	}
	blocksJSON, exists := payload["blocks"]
	if !exists {
		return nil
	}
	var blocks []structuralBlock
	if err := json.Unmarshal(blocksJSON, &blocks); err != nil {
		return fmt.Errorf("decode structural body for %s: %w", instance.ID, err)
	}
	expected := structuralPositionCount(blocks)
	if len(instance.Body.Positions) != expected {
		return fmt.Errorf("body position count mismatch for %s: expected %d, got %d", instance.ID, expected, len(instance.Body.Positions))
	}
	positionIndex := 0
	for blockIndex, block := range blocks {
		if block.Index != blockIndex {
			return fmt.Errorf("body block index mismatch for %s: %d", instance.ID, block.Index)
		}
		if block.Statements == nil || len(block.Terminator) == 0 || string(block.Terminator) == "null" {
			return fmt.Errorf("incomplete structural block for %s at index %d", instance.ID, blockIndex)
		}
		for statementIndex := range block.Statements {
			if err := validatePosition(instance, positionIndex, block.Index, &statementIndex, false); err != nil {
				return err
			}
			positionIndex++
		}
		if err := validatePosition(instance, positionIndex, block.Index, nil, true); err != nil {
			return err
		}
		positionIndex++
	}
	return nil
}
func structuralPositionCount(blocks []structuralBlock) int {
	count := 0
	for _, block := range blocks {
		count += len(block.Statements) + 1
	}
	return count
}

func validatePosition(instance Instance, index int, block int, statement *int, terminator bool) error {
	position := instance.Body.Positions[index]
	if !position.statementPresent || position.Block != block || !sameStatement(position.Statement, statement) || !position.terminatorMatches(terminator) {
		return fmt.Errorf("body position mismatch for %s at index %d", instance.ID, index)
	}
	return nil
}

func sameStatement(left *int, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
