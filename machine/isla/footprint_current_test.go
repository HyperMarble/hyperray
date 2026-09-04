// Current-input tests stop footprint work after either model artifact changes.
package isla_test

import (
	"os"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintRequestRejectsUnidentifiedArtifact(t *testing.T) {
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	request, err := isla.NewFootprintRequest(isla.FootprintRelease{}, instructions, 1, 1, 1)
	if err == nil {
		t.Fatalf("request = %#v, error = nil", request)
	}
	assertErrorCode(t, err, isla.ArtifactChanged)
}

func TestFootprintOperationRejectsChangedModelArtifact(t *testing.T) {
	cases := []string{"architecture", "configuration"}
	for index := range cases {
		t.Run(cases[index], func(t *testing.T) {
			report, err := changedModelOperation(t, index)
			assertFootprintError(t, report, err, isla.ArtifactChanged)
		})
	}
}

func changedModelOperation(t *testing.T, changedIndex int) (isla.FootprintReport, error) {
	t.Helper()
	paths := make([]string, 2)
	artifacts := make([]isla.Artifact, 2)
	for index := range artifacts {
		path, digest := artifactFixture(t, "model")
		paths[index] = path
		artifact, err := isla.NewArtifact(path, digest)
		if err != nil {
			t.Fatalf("NewArtifact() error = %v", err)
		}
		artifacts[index] = artifact
	}
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	engine := footprintEngine(t)
	release := footprintRelease(t, engine, artifacts[0], artifacts[1])
	request, err := isla.NewFootprintRequest(release, instructions, 1, 1, 4096)
	if err != nil {
		t.Fatalf("NewFootprintRequest() error = %v", err)
	}
	if err := os.WriteFile(paths[changedIndex], []byte("changed"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return engine.TraceInstructions(t.Context(), request)
}

func TestFootprintOperationRejectsNilContext(t *testing.T) {
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{1, 0}}}
	report, err := footprintEngine(t).TraceInstructions(nil, footprintRequest(t, instructions, 4096))
	assertFootprintError(t, report, err, isla.InvalidInput)
}
