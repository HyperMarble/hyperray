//go:build isla_integration

// Real-input functions identify external Isla files for the integration test.
// They must reject an absent path or a changed artifact.
package isla_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func realRequest(t *testing.T, fixture string) isla.Request {
	t.Helper()
	programPath, err := filepath.Abs(filepath.Join("../../fixtures/isla", fixture))
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	return realRequestPath(t, programPath)
}

func realRequestPath(t *testing.T, programPath string) isla.Request {
	return realRequestPathWithLimits(t, programPath, 2)
}

func realRequestPathWithLimits(t *testing.T, programPath string, pcVisitLimit uint64) isla.Request {
	t.Helper()
	architecture := realArtifact(t, "HYPERRAY_SAIL_IR")
	configuration := realArtifact(t, "HYPERRAY_ISLA_CONFIG")
	memoryModel := realArtifact(t, "HYPERRAY_MEMORY_MODEL")
	program := identifiedArtifact(t, programPath)
	request, err := isla.NewRequest(architecture, configuration, memoryModel, program, pcVisitLimit, 10, 16<<20)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	return request
}

func realArtifact(t *testing.T, variable string) isla.Artifact {
	t.Helper()
	return identifiedArtifact(t, requiredPath(t, variable))
}

func identifiedArtifact(t *testing.T, path string) isla.Artifact {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	digest := sha256.Sum256(content)
	artifact, err := isla.NewArtifact(path, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatalf("NewArtifact(%q) error = %v", path, err)
	}
	return artifact
}

func requiredPath(t *testing.T, variable string) string {
	t.Helper()
	path := os.Getenv(variable)
	if path == "" {
		t.Fatalf("environment variable %s is empty", variable)
	}
	return path
}
