//go:build isla_integration && rust_acceptance

// This helper creates the fixed Rust request for the acceptance test.
// It never accepts caller-selected source or output paths.
package isla_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type realRustBuildRequest struct {
	Driver           string           `json:"driver"`
	Linker           string           `json:"linker"`
	Source           string           `json:"source"`
	OutputDirectory  string           `json:"output_directory"`
	Target           string           `json:"target"`
	CompilerFlags    []string         `json:"compiler_flags"`
	LinkerFlags      []string         `json:"linker_flags"`
	BoundaryArtifact *string          `json:"boundary_artifact"`
	ExternArtifacts  []realRustExtern `json:"extern_artifacts"`
	Sysroot          string           `json:"sysroot"`
}

type realRustExtern struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func writeRealBuildRequest(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	request := realRustBuildRequest{
		Driver:          requiredPath(t, "HYPERRAY_MIR_DUMP"),
		Linker:          requiredPath(t, "HYPERRAY_RUST_LINKER"),
		Source:          realRustFixture(t, "arithmetic.rs"),
		OutputDirectory: filepath.Join(directory, "build"),
		Target:          "riscv64gc-unknown-linux-gnu",
		CompilerFlags:   []string{"--crate-type=lib", "--edition=2021", "-C", "opt-level=2", "-C", "panic=abort", "-C", "overflow-checks=off", "-C", "relocation-model=static", "-C", "code-model=medium"},
		LinkerFlags:     []string{"-m", "elf64lriscv", "-static", "-e", "_start", "--no-relax", "-T", realRustFixture(t, "layout.ld")},
		ExternArtifacts: []realRustExtern{},
		Sysroot:         requiredPath(t, "HYPERRAY_RUST_SYSROOT"),
	}
	content, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("encode compiler build request: %v", err)
	}
	path := filepath.Join(directory, "request.json")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write compiler build request: %v", err)
	}
	return path
}
