// Executed instructions must belong to the loaded static inventory.
// An instruction absent from one query remains part of static coverage.
package isla

import (
	"encoding/binary"
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

func matchFootprintQuery(report FootprintReport, request VerificationRequest) error {
	if report.Evidence.ArchitectureDigest != request.query.architecture.digest {
		return engineError(CoverageMismatch, "footprint architecture", "request digest differs")
	}
	if report.Evidence.ConfigurationDigest != request.query.configuration.digest {
		return engineError(CoverageMismatch, "footprint configuration", "request digest differs")
	}
	return nil
}

func matchProgramSemantics(program Program, report SemanticReport) error {
	declared := program.threadEntries
	if len(declared) == 0 && program.entryAddress != 0 {
		declared = []ThreadEntry{{EntryAddress: program.entryAddress}}
	}
	if !report.Complete || report.ThreadCount != uint64(len(declared)) {
		return engineError(CoverageMismatch, "executed instruction inventory", "reported thread count differs")
	}
	expected := expectedSemanticInstructions(program.instructions)
	if err := matchSemanticThreads(declared, report.Threads, report.Instructions, expected); err != nil {
		return err
	}
	actual := make(map[SemanticInstruction]struct{}, len(report.Instructions))
	for _, instruction := range report.Instructions {
		if _, exists := expected[instruction]; !exists {
			return semanticInventoryError("extra", instruction)
		}
		if _, exists := actual[instruction]; exists {
			return semanticInventoryError("duplicate", instruction)
		}
		actual[instruction] = struct{}{}
	}
	return nil
}

func expectedSemanticInstructions(instructions []machine.Instruction) map[SemanticInstruction]struct{} {
	result := make(map[SemanticInstruction]struct{}, len(instructions))
	for index := range instructions {
		instruction := instructions[index]
		result[SemanticInstruction{
			Address: instruction.Address, Encoding: machineSemanticEncoding(instruction.Bytes),
		}] = struct{}{}
	}
	return result
}

func machineSemanticEncoding(value []byte) string {
	encoding := uint64(binary.LittleEndian.Uint16(value))
	if len(value) == 4 {
		encoding = uint64(binary.LittleEndian.Uint32(value))
	}
	return fmt.Sprintf("%08x", encoding)
}

func semanticInventoryError(kind string, instruction SemanticInstruction) error {
	detail := fmt.Sprintf("%s instruction at %#x with encoding %s", kind, instruction.Address, instruction.Encoding)
	return engineError(CoverageMismatch, "executed instruction inventory", detail)
}
