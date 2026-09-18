//go:build isla_integration && rust_acceptance

// These helpers create one fixed Rust build request for the acceptance test.
// They never accept caller-selected source or output paths.
package isla_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/compilercatalog"
)

func runRealCompilerBuild(t *testing.T) compilercatalog.BuildManifest {
	t.Helper()
	requestPath := writeRealBuildRequest(t)
	output, err := exec.CommandContext(t.Context(), requiredPath(t, "HYPERRAY_COMPILER_BUILD_CLI"), "--request", requestPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiler build failed: %v\n%s", err, output)
	}
	var result struct {
		Manifest compilercatalog.BuildManifest `json:"manifest"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode compiler build result: %v", err)
	}
	return result.Manifest
}
func realRustFixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("../../fixtures/rust/machine", name))
	if err != nil {
		t.Fatalf("resolve Rust fixture: %v", err)
	}
	return path
}
