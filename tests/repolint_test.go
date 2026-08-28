// Tests for the repo-lint gate: the host repository's own configured linter
// decides, ray only detects the config and runs the tool. The four-way
// result is covered: no config, tool missing, findings, clean.
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/ray/internal/repolint"
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

func writeRepoLintFixture(t *testing.T, pyproject, source string) string {
	t.Helper()
	dir := t.TempDir()
	if pyproject != "" {
		if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "solution.py"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func requireRuff(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ruff"); err != nil {
		t.Skip("ruff is not installed on this machine")
	}
}

func TestRepoLint_NoConfigMeansNoGate(t *testing.T) {
	dir := writeRepoLintFixture(t, "", initWithoutDocstring)
	result := repolint.Check(dir, []string{"solution.py"})
	if result.Status != repolint.StatusNoConfig {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusNoConfig, result.Status, result.Output)
	}
}

func TestRepoLint_ConfiguredButToolMissingIsBlocked(t *testing.T) {
	dir := writeRepoLintFixture(t, ruffDocstringConfig, initWithoutDocstring)
	t.Setenv("PATH", dir)
	result := repolint.Check(dir, []string{"solution.py"})
	if result.Status != repolint.StatusBlocked {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusBlocked, result.Status, result.Output)
	}
}

func TestRepoLint_MissingInitDocstringIsAFinding(t *testing.T) {
	requireRuff(t)
	dir := writeRepoLintFixture(t, ruffDocstringConfig, initWithoutDocstring)
	result := repolint.Check(dir, []string{"solution.py"})
	if result.Status != repolint.StatusFindings {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusFindings, result.Status, result.Output)
	}
	if !strings.Contains(result.Output, "D107") {
		t.Fatalf("expected the tool's own rule ID D107 in the report, got: %s", result.Output)
	}
}

func TestRepoLint_DocumentedInitIsClean(t *testing.T) {
	requireRuff(t)
	dir := writeRepoLintFixture(t, ruffDocstringConfig, initWithDocstring)
	result := repolint.Check(dir, []string{"solution.py"})
	if result.Status != repolint.StatusClean {
		t.Fatalf("expected %q, got %q (%s)", repolint.StatusClean, result.Status, result.Output)
	}
}
