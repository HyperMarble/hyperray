// Public executable tests pass one generated ELF through all joined engines.
// They inspect source identity, static traces, dynamic coverage, and verdict.
package isla_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicExecutableVerifier(t *testing.T) {
	content := machineFixture(t)
	program, err := isla.BuildProgram(content, uint64(len(content)), executableBoundary(0x80100000, "~(0:x11 = 7)"))
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "generated.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	architecture := testArtifact(t, "architecture")
	configuration := testArtifact(t, "configuration")
	request := executableRequest(t, path, program, architecture, configuration)
	verifier := executableVerifier(t, architecture, configuration)
	result, err := verifier.VerifyProgram(t.Context(), request, program, executableLimits())
	if err != nil {
		t.Fatalf("VerifyProgram() error = %v", err)
	}
	if result.Verification.Status != isla.Proved {
		t.Errorf("Status = %q", result.Verification.Status)
	}
	if !reflect.DeepEqual(result.Program, program.Evidence()) || len(result.Footprints.Instructions) != 4 {
		t.Errorf("result evidence = %#v", result)
	}
}

func executableLimits() isla.ExecutableLimits {
	return isla.ExecutableLimits{ThreadLimit: 2, TimeLimitSeconds: 3, MaximumOutputBytes: 4096}
}

func executableRequest(t *testing.T, path string, program isla.Program, architecture isla.Artifact, configuration isla.Artifact) isla.VerificationRequest {
	t.Helper()
	programArtifact, err := isla.NewArtifact(path, program.Digest())
	if err != nil {
		t.Fatalf("NewArtifact() error = %v", err)
	}
	query, err := isla.NewRequest(
		architecture, configuration, testArtifact(t, "memory"), programArtifact, 2, 3, 4096,
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request, err := isla.NewVerificationRequest(query, 2, 64)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	return request
}
