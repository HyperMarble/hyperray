//go:build isla_integration

// Real footprint tests use the recorded Isla release set and public API.
// They require distinct model output for distinct instruction semantics.
package isla_test

import (
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealFootprintTracesLegalAndIllegalEncodings(t *testing.T) {
	engine, err := isla.NewFootprintEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA_FOOTPRINT"))
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	instructions := []machine.Instruction{
		{Address: 0x1000, Bytes: []byte{0x93, 0x02, 0x30, 0x00}},
		{Address: 0x1004, Bytes: []byte{0xff, 0xff, 0xff, 0xff}},
	}
	architecture := realArtifact(t, "HYPERRAY_SAIL_IR")
	configuration := realArtifact(t, "HYPERRAY_ISLA_CONFIG")
	release := footprintRelease(t, engine, architecture, configuration)
	request, err := isla.NewFootprintRequest(release, instructions, 2, 30, 2*1024*1024)
	if err != nil {
		t.Fatalf("NewFootprintRequest() error = %v", err)
	}
	report, err := engine.TraceInstructions(t.Context(), request)
	if err != nil {
		t.Fatalf("TraceInstructions() error = %v", err)
	}
	assertRealFootprints(t, report)
	fmt.Printf("traces=%d legal=%s illegal=%s\n", len(report.Instructions), report.Instructions[0].OutputDigest, report.Instructions[1].OutputDigest)
}

func assertRealFootprints(t *testing.T, report isla.FootprintReport) {
	t.Helper()
	if len(report.Instructions) != 2 {
		t.Fatalf("instruction count = %d", len(report.Instructions))
	}
	legal := report.Instructions[0]
	illegal := report.Instructions[1]
	if legal.TraceCount == 0 || illegal.TraceCount == 0 {
		t.Fatalf("trace counts = %d, %d", legal.TraceCount, illegal.TraceCount)
	}
	if legal.OutputDigest == illegal.OutputDigest {
		t.Error("legal and illegal semantics have one digest")
	}
}
