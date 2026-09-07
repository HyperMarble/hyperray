//go:build isla_integration

// Rust fixtures enter through the actual compiler and linker.
// Production code must not know fixture names or generated instruction values.
package isla_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func compileRustExecutable(t *testing.T, source string) []byte {
	t.Helper()
	compiler := requiredPath(t, "HYPERRAY_RUSTC")
	linker := requiredPath(t, "HYPERRAY_RUST_LINKER")
	directory := t.TempDir()
	object := filepath.Join(directory, "program.o")
	image := filepath.Join(directory, "program.elf")
	fixtures, err := filepath.Abs("../../fixtures/rust/machine")
	if err != nil {
		t.Fatal(err)
	}
	runRustBuildTool(t, compiler, "--version")
	runRustBuildTool(t, compiler, "--edition=2021", "--crate-type=lib",
		"--target=riscv64gc-unknown-linux-gnu", "--emit=obj", "-C", "opt-level=2",
		"-C", "panic=abort", "-C", "relocation-model=static", "-C", "code-model=medium",
		filepath.Join(fixtures, source), "-o", object)
	runRustBuildTool(t, linker, "--version")
	runRustBuildTool(t, linker, "-m", "elf64lriscv", "-static",
		"--no-relax", "-T", filepath.Join(fixtures, "layout.ld"), object, "-o", image)
	content, err := os.ReadFile(image)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func runRustBuildTool(t *testing.T, executable string, arguments ...string) {
	t.Helper()
	output, err := exec.CommandContext(t.Context(), executable, arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", executable, arguments, err, output)
	}
	if len(output) > 0 {
		t.Logf("%s", output)
	}
}
