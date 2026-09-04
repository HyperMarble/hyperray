// Coverage test support creates independent source and trace inventories.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func coverageFixture(t *testing.T) ([]machine.Instruction, isla.FootprintReport) {
	t.Helper()
	instructions := []machine.Instruction{
		{Address: 0x1000, Bytes: []byte{1, 0}},
		{Address: 0x1002, Bytes: []byte{2, 0}},
	}
	request := footprintRequest(t, instructions, 4096)
	report, err := footprintEngine(t).TraceInstructions(t.Context(), request)
	if err != nil {
		t.Fatalf("TraceInstructions() error = %v", err)
	}
	return instructions, report
}

func cloneFootprintReport(report isla.FootprintReport) isla.FootprintReport {
	result := report
	result.Instructions = append([]isla.InstructionTrace(nil), report.Instructions...)
	return result
}

func assertCoverageMismatch(t *testing.T, instructions []machine.Instruction, report isla.FootprintReport) {
	t.Helper()
	coverage, err := isla.ValidateFootprintInventory(instructions, report)
	if err == nil {
		t.Fatalf("coverage = %#v, error = nil", coverage)
	}
	assertErrorCode(t, err, isla.CoverageMismatch)
}
