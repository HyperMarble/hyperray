//go:build isla_integration

// Real diagnostic tests reject the stale upstream configuration warning.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealFootprintRejectsUnclassifiedDiagnostic(t *testing.T) {
	engine, err := isla.NewFootprintEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA_FOOTPRINT"))
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	architecture := realArtifact(t, "HYPERRAY_SAIL_IR")
	configuration := realArtifact(t, "HYPERRAY_ISLA_CONFIG")
	release := footprintRelease(t, engine, architecture, configuration)
	instructions := []machine.Instruction{
		{Address: 0x1000, Bytes: []byte{0x93, 0x02, 0x30, 0x00}},
	}
	request, err := isla.NewFootprintRequest(release, instructions, 2, 30, 2*1024*1024)
	if err != nil {
		t.Fatalf("NewFootprintRequest() error = %v", err)
	}
	report, err := engine.TraceInstructions(t.Context(), request)
	assertFootprintError(t, report, err, isla.ProtocolError)
}
