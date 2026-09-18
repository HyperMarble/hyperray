//go:build isla_integration && rust_acceptance

// These helpers load actual compiler artifacts and invoke the public verifier.
// They never replace missing tools or failed identities with fixtures.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/compilercatalog"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func realBuildObjects(t *testing.T, manifest compilercatalog.BuildManifest) []compilercatalog.ObjectArtifact {
	t.Helper()
	objects := []compilercatalog.ObjectArtifact{{Path: manifest.Object.Path, Content: readRealArtifact(t, manifest.Object.Path)}}
	for _, identity := range manifest.ExternArtifacts {
		objects = append(objects, compilercatalog.ObjectArtifact{Path: identity.Path, Content: readRealArtifact(t, identity.Path)})
	}
	if manifest.BoundaryArtifact != nil {
		identity := *manifest.BoundaryArtifact
		objects = append(objects, compilercatalog.ObjectArtifact{Path: identity.Path, Content: readRealArtifact(t, identity.Path)})
	}
	return objects
}

func verifyRealRustProgram(t *testing.T, program isla.Program) isla.ExecutableResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), "program.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatalf("write generated program: %v", err)
	}
	request, err := isla.NewRequest(realArtifact(t, "HYPERRAY_SAIL_IR"), realArtifact(t, "HYPERRAY_ISLA_CONFIG"), realArtifact(t, "HYPERRAY_MEMORY_MODEL"), identifiedArtifact(t, path), 2, 10, 16<<20)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	verification, err := isla.NewVerificationRequest(request, 1, 256)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	result, err := realExecutableVerifier(t).VerifyProgram(t.Context(), verification, program, isla.ExecutableLimits{ThreadLimit: 1, TimeLimitSeconds: 10, MaximumOutputBytes: 16 << 20})
	if err != nil {
		t.Fatalf("VerifyProgram() error = %v", err)
	}
	return result
}

func readRealArtifact(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}
