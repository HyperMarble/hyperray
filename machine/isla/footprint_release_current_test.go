// Release current-state tests reject changed manifests and different engines.
package isla_test

import (
	"os"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintReleaseRejectsChangedManifest(t *testing.T) {
	engine, architecture, configuration := releaseInputs(t)
	manifest := manifestArtifact(t, matchingManifest(engine, architecture, configuration))
	content, err := os.ReadFile(manifest.Path())
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if err := os.WriteFile(manifest.Path(), append(content, '\n'), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	release, err := isla.NewFootprintRelease(manifest, engine, architecture, configuration)
	if err == nil {
		t.Fatalf("release = %#v, error = nil", release)
	}
	assertErrorCode(t, err, isla.ArtifactChanged)
}

func TestFootprintOperationRejectsDifferentReleaseEngine(t *testing.T) {
	engine, architecture, configuration := releaseInputs(t)
	release := footprintRelease(t, engine, architecture, configuration)
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	request, err := isla.NewFootprintRequest(release, instructions, 1, 1, 4096)
	if err != nil {
		t.Fatalf("NewFootprintRequest() error = %v", err)
	}
	otherPath := temporaryTool(t, "printf '%s\\n' different")
	otherEngine, err := isla.NewFootprintEngine(t.Context(), otherPath)
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	report, err := otherEngine.TraceInstructions(t.Context(), request)
	assertFootprintError(t, report, err, isla.ReleaseMismatch)
}
