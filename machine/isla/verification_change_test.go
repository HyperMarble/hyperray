// Change tests require post-operation identity checks to reject modified tools.
// The mutation occurs only in a temporary copy of the fixture executable.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestVerifierRejectsSemanticToolChangedDuringOperation(t *testing.T) {
	source := semanticTool(t, "fake-isla-litmus-dump.sh")
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "semantic-tool")
	if err := os.WriteFile(path, content, 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	semantics, err := isla.NewSemanticEngine(t.Context(), path)
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	verifier, err := isla.NewVerifier(testEngine(t), semantics)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	request := verificationRequest(t, "semantic-change-tool", 4096)
	result, err := verifier.Verify(t.Context(), request)
	assertVerificationError(t, result, err, isla.ToolChanged)
}
