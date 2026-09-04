// Request tests construct every production input through the public API.
// They must not change the package to use a different program.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicRequest(t *testing.T) {
	request := testRequest(t, "proof")
	engine := testEngine(t)
	proposal, err := engine.Propose(nil, request)
	assertProposalError(t, proposal, err, isla.InvalidInput)
}

func TestRequestRejectsZeroLimits(t *testing.T) {
	artifact := testArtifact(t, "input")
	request, err := isla.NewRequest(artifact, artifact, artifact, artifact, 0, 1)
	if err == nil {
		t.Errorf("NewRequest() = %#v, nil error", request)
	}
}

func testRequest(t *testing.T, program string) isla.Request {
	t.Helper()
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	memoryModel := testArtifact(t, "memory-model")
	programArtifact := testArtifact(t, program)
	request, err := isla.NewRequest(architecture, configuration, memoryModel, programArtifact, 2, 3)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	return request
}

func testArtifact(t *testing.T, content string) isla.Artifact {
	t.Helper()
	path, digest := artifactFixture(t, content)
	artifact, err := isla.NewArtifact(path, digest)
	if err != nil {
		t.Fatalf("NewArtifact() error = %v", err)
	}
	return artifact
}

func testEngine(t *testing.T) isla.Engine {
	t.Helper()
	engine, err := isla.NewEngine(t.Context(), fixtureTool(t))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return engine
}
