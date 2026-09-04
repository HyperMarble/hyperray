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
	configuration := realFootprintConfiguration(t)
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
	coverage, err := isla.ValidateFootprintInventory(instructions, report)
	if err != nil {
		t.Fatalf("ValidateFootprintInventory() error = %v", err)
	}
	fmt.Printf("complete=%t traces=%d dispositions=%d/%d legal=%s illegal=%s\n", coverage.Complete, len(report.Instructions), len(report.Instructions[0].Dispositions), len(report.Instructions[1].Dispositions), report.Instructions[0].OutputDigest, report.Instructions[1].OutputDigest)
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
	if len(legal.Dispositions) == 0 || len(illegal.Dispositions) == 0 {
		t.Error("real missing-primitive diagnostics lack dispositions")
	}
}
