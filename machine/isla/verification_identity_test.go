// Identity tests stop verification when its Isla tools cannot share a release.
// They exercise public construction instead of changing private state.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestVerifierRejectsToolVersionMismatch(t *testing.T) {
	semantics, err := isla.NewSemanticEngine(t.Context(), semanticTool(t, "fake-isla-litmus-dump-mismatch.sh"))
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	verifier, err := isla.NewVerifier(testEngine(t), semantics)
	if err == nil {
		t.Errorf("NewVerifier() = %#v, nil error", verifier)
		return
	}
	assertErrorCode(t, err, isla.ReleaseMismatch)
}

func TestProposalRejectsUnknownSuccessfulDiagnostic(t *testing.T) {
	proposal, err := testEngine(t).Propose(t.Context(), testRequest(t, "proposal-warning"))
	assertProposalError(t, proposal, err, isla.ProtocolError)
}

func TestPublicSemanticEngineIdentity(t *testing.T) {
	engine, err := isla.NewSemanticEngine(t.Context(), semanticTool(t, "fake-isla-litmus-dump.sh"))
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	identity := engine.Identity()
	if identity.Version != "v0.2.0/test" || len(identity.Digest) != 64 {
		t.Errorf("Identity() = %#v", identity)
	}
	missing, err := isla.NewSemanticEngine(t.Context(), "/missing/hyperray-isla-dump")
	if err == nil {
		t.Errorf("NewSemanticEngine() = %#v, nil error", missing)
	}
}

func TestVerifierRejectsUnidentifiedTools(t *testing.T) {
	semantics, err := isla.NewSemanticEngine(t.Context(), semanticTool(t, "fake-isla-litmus-dump.sh"))
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	verifier, err := isla.NewVerifier(isla.Engine{}, semantics)
	if err == nil {
		t.Errorf("NewVerifier() = %#v, nil error", verifier)
	}
	verifier, err = isla.NewVerifier(testEngine(t), isla.SemanticEngine{})
	if err == nil {
		t.Errorf("NewVerifier() = %#v, nil error", verifier)
	}
}
