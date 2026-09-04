// Diagnostic tests accept only missing primitives not reached by completed execution.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintRecordsUnreachedPrimitive(t *testing.T) {
	instructions := []machine.Instruction{
		{Address: 0x1000, Bytes: []byte{0xab, 0xab, 0xab, 0xab}},
	}
	report, err := footprintEngine(t).TraceInstructions(t.Context(), footprintRequest(t, instructions, 4096))
	if err != nil {
		t.Fatalf("TraceInstructions() error = %v", err)
	}
	dispositions := report.Instructions[0].Dispositions
	if len(dispositions) != 1 {
		t.Fatalf("disposition count = %d", len(dispositions))
	}
	disposition := dispositions[0]
	if disposition.Kind != isla.UnavailablePrimitive || disposition.Disposition != isla.NotCalledInCompletedExecution {
		t.Errorf("disposition = %#v", disposition)
	}
	coverage, err := isla.ValidateFootprintInventory(instructions, report)
	if err != nil || !coverage.Complete {
		t.Errorf("coverage = %#v, error = %v", coverage, err)
	}
}
