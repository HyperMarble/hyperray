// Current-input tests change accepted files before the proof process starts.
// They require each change to stop the operation.
package isla_test

import (
	"os"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestZeroEngineCannotOperate(t *testing.T) {
	proposal, err := (isla.Engine{}).Propose(t.Context(), testRequest(t, "proof"))
	assertProposalError(t, proposal, err, isla.ToolIdentityFail)
}

func TestChangedEngineCannotOperate(t *testing.T) {
	path := temporaryTool(t, "printf '%s\\n' v1")
	engine, err := isla.NewEngine(t.Context(), path)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	proposal, err := engine.Propose(t.Context(), testRequest(t, "proof"))
	assertProposalError(t, proposal, err, isla.ToolChanged)
}

func TestRemovedEngineCannotOperate(t *testing.T) {
	path := temporaryTool(t, "printf '%s\\n' v1")
	engine, err := isla.NewEngine(t.Context(), path)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove() error = %v", err)
	}
	proposal, err := engine.Propose(t.Context(), testRequest(t, "proof"))
	assertProposalError(t, proposal, err, isla.ToolChanged)
}

func TestChangedProgramCannotOperate(t *testing.T) {
	path, digest := artifactFixture(t, "proof")
	program, err := isla.NewArtifact(path, digest)
	if err != nil {
		t.Fatalf("NewArtifact() error = %v", err)
	}
	input := testArtifact(t, "input")
	request, err := isla.NewRequest(input, input, input, program, 1, 1)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	proposal, err := testEngine(t).Propose(t.Context(), request)
	assertProposalError(t, proposal, err, isla.ArtifactChanged)
}
