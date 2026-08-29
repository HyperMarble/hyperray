// Tests for the repo-lint gate: the host repository's own configured
// linters decide, hyperray only detects the config and runs the tools. Covered
// per language: no config, tool missing, findings, clean, and the rule that
// a configured linter stays quiet when the solution never touches its
// language.
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/repolint"
)

const ruffDocstringConfig = "[tool.ruff.lint]\nselect = [\"D107\"]\n"

const initWithoutDocstring = `class Composed:
    def __init__(self, parts):
        self.parts = parts
`

const initWithDocstring = `class Composed:
    def __init__(self, parts):
        """Store the component parts.

        Parameters
        ----------
        parts : list
            The components to compose.
        """
        self.parts = parts
`

func writeRepoLintFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireTool(t *testing.T, binary string) {
	t.Helper()
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("%s is not installed on this machine", binary)
	}
}

func onlyResult(t *testing.T, results []repolint.Result) repolint.Result {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("expected exactly one gate result, got %d: %+v", len(results), results)
	}
	return results[0]
}

func TestRepoLint_NoConfigMeansNoGate(t *testing.T) {
	dir := t.TempDir()
	writeRepoLintFile(t, dir, "solution.py", initWithoutDocstring)
	if results := repolint.Check(dir, []string{"solution.py"}); len(results) != 0 {
		t.Fatalf("expected no gates, got %+v", results)
	}
}

func TestRepoLint_ConfiguredButUntouchedLanguageStaysQuiet(t *testing.T) {
	dir := t.TempDir()
	writeRepoLintFile(t, dir, "pyproject.toml", ruffDocstringConfig)
	writeRepoLintFile(t, dir, "solution.rs", "fn main() {}\n")
	if results := repolint.Check(dir, []string{"solution.rs"}); len(results) != 0 {
		t.Fatalf("expected the Python gate to stay quiet for a Rust solution, got %+v", results)
	}
}

func TestRepoLint_ConfiguredButToolMissingIsBlocked(t *testing.T) {
	dir := t.TempDir()
	writeRepoLintFile(t, dir, "pyproject.toml", ruffDocstringConfig)
	writeRepoLintFile(t, dir, "solution.py", initWithoutDocstring)
	t.Setenv("PATH", dir)
	result := onlyResult(t, repolint.Check(dir, []string{"solution.py"}))
	if result.Status != repolint.StatusBlocked {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusBlocked, result.Status, result.Output)
	}
}

func TestRepoLint_MissingInitDocstringIsAFinding(t *testing.T) {
	requireTool(t, "ruff")
	dir := t.TempDir()
	writeRepoLintFile(t, dir, "pyproject.toml", ruffDocstringConfig)
	writeRepoLintFile(t, dir, "solution.py", initWithoutDocstring)
	result := onlyResult(t, repolint.Check(dir, []string{"solution.py"}))
	if result.Status != repolint.StatusFindings {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusFindings, result.Status, result.Output)
	}
	if !strings.Contains(result.Output, "D107") {
		t.Fatalf("expected the tool's own rule ID D107 in the report, got: %s", result.Output)
	}
}

func TestRepoLint_DocumentedInitIsClean(t *testing.T) {
	requireTool(t, "ruff")
	dir := t.TempDir()
	writeRepoLintFile(t, dir, "pyproject.toml", ruffDocstringConfig)
	writeRepoLintFile(t, dir, "solution.py", initWithDocstring)
	result := onlyResult(t, repolint.Check(dir, []string{"solution.py"}))
	if result.Status != repolint.StatusClean {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusClean, result.Status, result.Output)
	}
}

func writeClippyCrate(t *testing.T, mainBody string) string {
	t.Helper()
	dir := t.TempDir()
	writeRepoLintFile(t, dir, "Cargo.toml", `[package]
name = "gatecheck"
version = "0.1.0"
edition = "2021"

[lints.clippy]
needless_return = "deny"
`)
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoLintFile(t, dir, filepath.Join("src", "main.rs"), mainBody)
	return dir
}

func TestRepoLint_ClippyDeniedLintIsAFinding(t *testing.T) {
	requireTool(t, "cargo")
	dir := writeClippyCrate(t, "fn answer() -> i32 {\n    return 42;\n}\n\nfn main() {\n    println!(\"{}\", answer());\n}\n")
	result := onlyResult(t, repolint.Check(dir, []string{"src/main.rs"}))
	if result.Status != repolint.StatusFindings {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusFindings, result.Status, result.Output)
	}
	if !strings.Contains(result.Output, "needless_return") {
		t.Fatalf("expected the denied lint name in the report, got: %s", result.Output)
	}
}

func TestRepoLint_ClippyCleanCratePasses(t *testing.T) {
	requireTool(t, "cargo")
	dir := writeClippyCrate(t, "fn main() {\n    println!(\"ok\");\n}\n")
	result := onlyResult(t, repolint.Check(dir, []string{"src/main.rs"}))
	if result.Status != repolint.StatusClean {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusClean, result.Status, result.Output)
	}
}

func TestRepoLint_ClangTidyWithoutCompileCommandsIsBlocked(t *testing.T) {
	dir := t.TempDir()
	writeRepoLintFile(t, dir, ".clang-tidy", "Checks: 'readability-*'\n")
	writeRepoLintFile(t, dir, "solution.cpp", "int main() { return 0; }\n")
	result := onlyResult(t, repolint.Check(dir, []string{"solution.cpp"}))
	if result.Status != repolint.StatusBlocked {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusBlocked, result.Status, result.Output)
	}
	if !strings.Contains(result.Output, "compile_commands.json") {
		t.Fatalf("expected the blocking reason to name compile_commands.json, got: %s", result.Output)
	}
}

func TestRepoLint_ClangFormatViolationIsAFinding(t *testing.T) {
	requireTool(t, "clang-format")
	dir := t.TempDir()
	writeRepoLintFile(t, dir, ".clang-format", "BasedOnStyle: LLVM\n")
	writeRepoLintFile(t, dir, "solution.cpp", "int   main( ){return 0 ;}\n")
	result := onlyResult(t, repolint.Check(dir, []string{"solution.cpp"}))
	if result.Status != repolint.StatusFindings {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusFindings, result.Status, result.Output)
	}
}

func TestBundle_PatchFilesAndMissing(t *testing.T) {
	patch := "diff --git a/test.sh b/test.sh\nnew file mode 100755\n@@ -0,0 +1 @@\n+echo hi\ndiff --git a/pkg/tests/test_x.py b/pkg/tests/test_x.py\n@@ -1 +1 @@\n-a\n+b\n"
	files := repolint.PatchFiles(patch)
	if len(files) != 2 || files[0] != "test.sh" || files[1] != "pkg/tests/test_x.py" {
		t.Fatalf("unexpected files: %v", files)
	}
	if missing := repolint.MissingBundleFiles(patch, []string{"test.sh"}); len(missing) != 0 {
		t.Fatalf("test.sh is delivered, got missing=%v", missing)
	}
	missing := repolint.MissingBundleFiles(patch, []string{"test.sh", "run.sh"})
	if len(missing) != 1 || missing[0] != "run.sh" {
		t.Fatalf("expected run.sh missing, got %v", missing)
	}
}
