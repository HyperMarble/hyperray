// End-to-end proof that the Rust path works against real cargo: the runner
// lists the fixture crate's tests, and a raise breaker derived from the
// panic-message rule makes exactly the guarding test fail, with the failure
// name read from cargo's own report.
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/enforce"
	"github.com/HyperMarble/hyperray/internal/runner"
)

func TestRustEndToEnd_ListBreakAndParse(t *testing.T) {
	requireTool(t, "cargo")
	crate := copyRustFixture(t)
	rustRunner, err := runner.New("rust", "", "", "cargo test --quiet")
	if err != nil {
		t.Fatal(err)
	}

	listing := runShell(t, crate, rustRunner.ListCommand())
	if !strings.Contains(listing, "tests::sums_components") || !strings.Contains(listing, "tests::rejects_empty_with_message") {
		t.Fatalf("listing missed fixture tests: %s", listing)
	}

	source := filepath.Join(crate, "src", "lib.rs")
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(original), "at least one component", "hyperray broke this message", 1)
	if err := os.WriteFile(source, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	output := runShellAllowFail(t, crate, rustRunner.SuiteCommand())
	names := rustRunner.FailedNames(output)
	if len(names) != 1 || names[0] != "tests::rejects_empty_with_message" {
		t.Fatalf("expected exactly the guarding test to fail, got %v in: %s", names, output)
	}

	suppressed := enforce.SuppressRaise("rust", string(original), "ValueError", "at least one component")
	if suppressed == "" {
		t.Fatal("no-raise breaker was not derivable")
	}
	if err := os.WriteFile(source, []byte(suppressed), 0o644); err != nil {
		t.Fatal(err)
	}
	output = runShellAllowFail(t, crate, rustRunner.SuiteCommand())
	if names := rustRunner.FailedNames(output); len(names) == 0 {
		t.Fatalf("suppressing the panic must fail a test, got clean run: %s", output)
	}
}

func copyRustFixture(t *testing.T) string {
	t.Helper()
	crate := t.TempDir()
	for _, name := range []string{"Cargo.toml", filepath.Join("src", "lib.rs")} {
		body, err := os.ReadFile(filepath.Join("..", "testdata", "rustcrate", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(crate, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(crate, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return crate
}

func runShell(t *testing.T, dir, command string) string {
	t.Helper()
	sub := exec.Command("sh", "-c", command)
	sub.Dir = dir
	output, err := sub.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s\n%s", command, output)
	}
	return string(output)
}

func runShellAllowFail(t *testing.T, dir, command string) string {
	t.Helper()
	sub := exec.Command("sh", "-c", command)
	sub.Dir = dir
	output, _ := sub.CombinedOutput()
	return string(output)
}
