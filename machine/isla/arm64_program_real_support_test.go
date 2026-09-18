//go:build isla_integration && arm64_acceptance

// This helper builds the approved ARM64 boundary and real verification request.
// It must not substitute a fake engine or a different machine profile.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func verifyARM64Program(t *testing.T, program isla.Program) isla.ExecutableResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), "arm64-program.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	request := arm64VerificationRequest(t, path, program)
	result, err := arm64ExecutableVerifier(t).VerifyProgram(t.Context(), request, program, isla.ExecutableLimits{
		ThreadLimit: 1, TimeLimitSeconds: 60, MaximumOutputBytes: 32 << 20,
	})
	if err != nil {
		t.Fatalf("VerifyProgram() error = %v", err)
	}
	return result
}

func arm64VerificationRequest(t *testing.T, path string, program isla.Program) isla.VerificationRequest {
	t.Helper()
	return arm64VerificationRequestWithOutputLimit(t, path, program, 32<<20)
}

func arm64VerificationRequestWithOutputLimit(t *testing.T, path string, program isla.Program, maximumOutputBytes uint64) isla.VerificationRequest {
	t.Helper()
	programArtifact, err := isla.NewArtifact(path, program.Digest())
	if err != nil {
		t.Fatalf("NewArtifact() error = %v", err)
	}
	query, err := isla.NewRequest(
		arm64Artifact(t, "HYPERRAY_ARM64_SAIL_IR"),
		arm64Artifact(t, "HYPERRAY_ARM64_ISLA_CONFIG"),
		arm64Artifact(t, "HYPERRAY_ARM64_MEMORY_MODEL"),
		programArtifact, 256, 60, maximumOutputBytes,
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request, err := isla.NewVerificationRequest(query, 1, 2048)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	return request
}

func arm64Artifact(t *testing.T, variable string) isla.Artifact {
	t.Helper()
	path := requiredPath(t, variable)
	if variable == "HYPERRAY_ARM64_SAIL_IR" {
		artifact, err := isla.NewArtifact(path, "4ece4410a43c32b58737957d55920b485ade5171bb53deddebb9c7d61df77075")
		if err != nil {
			t.Fatalf("NewArtifact(%s) error = %v", variable, err)
		}
		return artifact
	}
	return identifiedArtifact(t, path)
}
