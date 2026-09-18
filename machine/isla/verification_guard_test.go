// Guard tests exercise internal invariants that public constructors preserve.
// Invalid identities, requests, and durations must stop before execution.
package isla

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func currentTestIdentity(t *testing.T, version string) ToolIdentity {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(path, []byte("tool"), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	digest, err := fileDigest(path)
	if err != nil {
		t.Fatalf("fileDigest() error = %v", err)
	}
	return ToolIdentity{Path: path, Version: version, Digest: digest}
}

func requireInternalError(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Errorf("error = %v, want %q", err, code)
	}
}

func TestVerifierGuardsCurrentToolsAndRequest(t *testing.T) {
	identity := currentTestIdentity(t, "v1")
	request := VerificationRequest{}
	result, err := (Verifier{}).Verify(t.Context(), request)
	requireInternalError(t, err, ToolIdentityFail)
	verifier := Verifier{solver: Engine{identity: identity}}
	result, err = verifier.Verify(t.Context(), request)
	requireInternalError(t, err, ToolIdentityFail)
	other := identity
	other.Version = "v2"
	verifier = Verifier{solver: Engine{identity: identity}, semantics: SemanticEngine{identity: other}}
	result, err = verifier.Verify(t.Context(), request)
	requireInternalError(t, err, ReleaseMismatch)
	verifier.semantics.identity.Version = "v1"
	result, err = verifier.Verify(t.Context(), request)
	requireInternalError(t, err, ArtifactChanged)
	if result.Status != "" {
		t.Errorf("failure result = %#v", result)
	}
}

func TestOperationsRejectUnrepresentableDuration(t *testing.T) {
	query := Request{timeLimit: ^uint64(0)}
	_, err := (Engine{}).operate(t.Context(), query)
	requireInternalError(t, err, InvalidInput)
	request := VerificationRequest{query: query}
	_, err = (SemanticEngine{}).runSemanticDump(t.Context(), request)
	requireInternalError(t, err, InvalidInput)
}
