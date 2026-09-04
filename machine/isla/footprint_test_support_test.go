// Footprint test support builds public requests from caller-owned instructions.
// It uses no package-private construction path.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func footprintEngine(t *testing.T) isla.FootprintEngine {
	t.Helper()
	engine, err := isla.NewFootprintEngine(t.Context(), footprintTool(t))
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	return engine
}

func footprintTool(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../fixtures/isla/fake-isla-footprint.sh")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("os.Chmod() error = %v", err)
	}
	return path
}

func footprintRequest(t *testing.T, instructions []machine.Instruction, outputLimit uint64) isla.FootprintRequest {
	t.Helper()
	engine := footprintEngine(t)
	architecture := testArtifact(t, "footprint-architecture")
	configuration := testArtifact(t, "footprint-configuration")
	release := footprintRelease(t, engine, architecture, configuration)
	request, err := isla.NewFootprintRequest(release, instructions, 2, 3, outputLimit)
	if err != nil {
		t.Fatalf("NewFootprintRequest() error = %v", err)
	}
	return request
}

func defaultFootprintRelease(t *testing.T) isla.FootprintRelease {
	t.Helper()
	engine := footprintEngine(t)
	architecture := testArtifact(t, "default-footprint-architecture")
	configuration := testArtifact(t, "default-footprint-configuration")
	return footprintRelease(t, engine, architecture, configuration)
}

func assertFootprintError(t *testing.T, report isla.FootprintReport, err error, code isla.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("report = %#v, error = nil", report)
	}
	assertErrorCode(t, err, code)
}
