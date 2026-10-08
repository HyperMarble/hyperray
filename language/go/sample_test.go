// Purpose: makes a tiny real Go module on disk for one test, and reads the
// symbols of a built file to see which functions have machine code.
// Never:   shares a module folder between tests; each test gets its own.
package goadapter_test

import (
	"debug/macho"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const sampleModule = "module example.com/sample\n\ngo 1.25\n"

// writeModule writes go.mod and the given files under a fresh folder, and
// returns that folder and an empty output folder next to it.
func writeModule(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	base := t.TempDir()
	root, out := filepath.Join(base, "module"), filepath.Join(base, "out")
	files["go.mod"] = sampleModule
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, out
}

// symbols lists the names of every function that has its own machine code
// in the built file.
func symbols(t *testing.T, path string) []string {
	t.Helper()
	file, err := macho.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	names := []string{}
	for _, symbol := range file.Symtab.Syms {
		names = append(names, symbol.Name)
	}
	return names
}

func hasSymbol(names []string, suffix string) bool {
	for _, name := range names {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// gitCommit makes the folder a committed git repository, so Go records the
// commit in the built file as a real project's build would.
func gitCommit(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "sample"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatal(string(output))
		}
	}
}
