// End-to-end proof that generated probes run in Rust and C++: the harness
// compiles against a filled entry, a wrapper script invokes it with a
// rule's key=value conditions, and both the success path and the error
// path print the shared observation shape.
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/bridges"
)

const rustFilledEntry = `pub fn run(operation: &str, values: &[(String, String)]) -> String {
    let parts = values.iter().find(|(key, _)| key == "parts").map(|(_, value)| value.as_str()).unwrap_or("");
    if parts == "empty" {
        panic!("at least one component");
    }
    format!("{operation}: composed {parts}")
}
`

const cppFilledEntry = `#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

std::string run(const std::string &operation,
                const std::vector<std::pair<std::string, std::string>> &values) {
    std::string parts;
    for (const auto &pair : values) {
        if (pair.first == "parts") parts = pair.second;
    }
    if (parts == "empty") {
        throw std::invalid_argument("at least one component");
    }
    return operation + ": composed " + parts;
}
`

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRustProbes_ObserveSuccessAndPanic(t *testing.T) {
	requireTool(t, "cargo")
	crate := copyRustFixture(t)
	writeExecutable(t, filepath.Join(crate, "examples", "hyperray_probes.rs"), bridges.RustHarness())
	writeExecutable(t, filepath.Join(crate, "examples", "hyperray_entry.rs"), rustFilledEntry)
	bridgesDir := filepath.Join(crate, "bridges")

	ok := filepath.Join(bridgesDir, "wit_compose__two.sh")
	writeExecutable(t, ok, bridges.WrapperScript("rust", "compose", map[string]string{"parts": "two"}))
	output := runShell(t, crate, "sh "+ok)
	if strings.TrimSpace(output) != "compose: composed two" {
		t.Fatalf("unexpected observation: %q", output)
	}

	raise := filepath.Join(bridgesDir, "wit_compose__empty.sh")
	writeExecutable(t, raise, bridges.WrapperScript("rust", "compose", map[string]string{"parts": "empty"}))
	output = runShell(t, crate, "sh "+raise)
	if !strings.Contains(output, "panic: at least one component") {
		t.Fatalf("panic path must print the message: %q", output)
	}
}

func TestCppProbes_ObserveSuccessAndException(t *testing.T) {
	requireTool(t, "c++")
	root := t.TempDir()
	bridgesDir := filepath.Join(root, "bridges")
	writeExecutable(t, filepath.Join(bridgesDir, "hyperray_probes.cpp"), bridges.CppHarness())
	writeExecutable(t, filepath.Join(bridgesDir, "hyperray_entry.cpp"), cppFilledEntry)

	ok := filepath.Join(bridgesDir, "wit_compose__two.sh")
	writeExecutable(t, ok, bridges.WrapperScript("cpp", "compose", map[string]string{"parts": "two"}))
	output := runShell(t, root, "sh "+ok)
	if strings.TrimSpace(output) != "compose: composed two" {
		t.Fatalf("unexpected observation: %q", output)
	}

	raise := filepath.Join(bridgesDir, "wit_compose__empty.sh")
	writeExecutable(t, raise, bridges.WrapperScript("cpp", "compose", map[string]string{"parts": "empty"}))
	output = runShell(t, root, "sh "+raise)
	if !strings.Contains(output, "invalid_argument: at least one component") {
		t.Fatalf("exception path must print demangled type and message: %q", output)
	}
}
