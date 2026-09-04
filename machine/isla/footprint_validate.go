// Footprint validation compares caller and engine inventories independently.
// It accepts no count-only or one-direction match.
package isla

import (
	"encoding/hex"
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

// ValidateFootprintInventory accepts only exact bidirectional trace coverage.
func ValidateFootprintInventory(instructions []machine.Instruction, report FootprintReport) (FootprintCoverage, error) {
	expected, err := copyInstructions(instructions)
	if err != nil {
		return FootprintCoverage{}, err
	}
	if err := validateFootprintEvidence(report.Evidence); err != nil {
		return FootprintCoverage{}, err
	}
	actual, err := traceInventory(report.Instructions)
	if err != nil {
		return FootprintCoverage{}, err
	}
	for index := range expected {
		instruction := expected[index]
		trace, exists := actual[instruction.Address]
		if !exists {
			return FootprintCoverage{}, footprintCoverageError("missing", instruction.Address)
		}
		encoding := hex.EncodeToString(instruction.Bytes)
		if trace.Encoding != encoding {
			return FootprintCoverage{}, footprintCoverageError("changed", instruction.Address)
		}
	}
	if err := rejectExtraTraces(expected, report.Instructions); err != nil {
		return FootprintCoverage{}, err
	}
	count := uint64(len(expected))
	return FootprintCoverage{Complete: true, CoveredInstructions: count, TotalInstructions: count}, nil
}

func rejectExtraTraces(expected []machine.Instruction, traces []InstructionTrace) error {
	addresses := make(map[uint64]struct{}, len(expected))
	for index := range expected {
		addresses[expected[index].Address] = struct{}{}
	}
	for index := range traces {
		if _, exists := addresses[traces[index].Address]; !exists {
			return footprintCoverageError("extra", traces[index].Address)
		}
	}
	return nil
}

func footprintCoverageError(kind string, address uint64) error {
	detail := fmt.Sprintf("%s instruction trace at %#x", kind, address)
	return engineError(CoverageMismatch, "footprint inventory", detail)
}
