// Verification test support constructs every value through the public API.
// It uses identified fixture tools and caller-visible errors.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func testVerifier(t *testing.T) isla.Verifier {
	t.Helper()
	solver := testEngine(t)
	semantics, err := isla.NewSemanticEngine(t.Context(), semanticTool(t, "fake-isla-litmus-dump.sh"))
	if err != nil {
		t.Fatalf("NewSemanticEngine() error = %v", err)
	}
	verifier, err := isla.NewVerifier(solver, semantics)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	return verifier
}

func semanticTool(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("../../fixtures/isla", name))
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("os.Chmod() error = %v", err)
	}
	return path
}

func verificationRequest(t *testing.T, program string, outputLimit uint64) isla.VerificationRequest {
	t.Helper()
	query := testRequestWithLimits(t, program, 3, outputLimit)
	request, err := isla.NewVerificationRequest(query, 2, 64)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	return request
}

func assertVerificationError(t *testing.T, result isla.VerificationResult, err error, code isla.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("result = %#v, error = nil", result)
	}
	if result.Status != "" || result.Semantics.Complete {
		t.Errorf("failure returned result %#v", result)
	}
	assertErrorCode(t, err, code)
}
