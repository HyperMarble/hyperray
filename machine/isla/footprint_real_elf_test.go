//go:build isla_integration

// Real ELF coverage tests connect the public loader to the public Isla route.
package isla_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealLoadedELFHasEveryInstructionTrace(t *testing.T) {
	image := realMachineImage(t)
	engine, err := isla.NewFootprintEngine(t.Context(), requiredPath(t, "HYPERRAY_ISLA_FOOTPRINT"))
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	architecture := realArtifact(t, "HYPERRAY_SAIL_IR")
	configuration := realFootprintConfiguration(t)
	release := footprintRelease(t, engine, architecture, configuration)
	request, err := isla.NewFootprintRequest(release, image.Instructions, 2, 60, 4*1024*1024)
	if err != nil {
		t.Fatalf("NewFootprintRequest() error = %v", err)
	}
	report, err := engine.TraceInstructions(t.Context(), request)
	if err != nil {
		t.Fatalf("TraceInstructions() error = %v", err)
	}
	coverage, err := isla.ValidateFootprintInventory(image.Instructions, report)
	if err != nil {
		t.Fatalf("ValidateFootprintInventory() error = %v", err)
	}
	fmt.Printf("elf_instructions=%d covered=%d complete=%t\n", len(image.Instructions), coverage.CoveredInstructions, coverage.Complete)
}

func realMachineImage(t *testing.T) machine.Image {
	t.Helper()
	path := filepath.Join("../../fixtures/machine", "rv64-lp64d-static.elf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	image, err := machine.Load(content, uint64(len(content)))
	if err != nil {
		t.Fatalf("machine.Load() error = %v", err)
	}
	return image
}
