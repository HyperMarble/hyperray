// Purpose: a test program Go rejects does not erase an executable the build
// already made or the reason the test program was not built.
// Never:   turns a successful program build into a blocked outcome.
package goadapter_test

import (
	"os"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const genericTest = "package main\n\nimport \"testing\"\n\nfunc TestGeneric[T any](t *testing.T) {}\n"

func TestARejectedTestProgramDoesNotEraseABuiltExecutable(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"main.go":      "package main\n\nfunc main() {}\n",
		"main_test.go": genericTest,
	})
	outcome := goadapter.BuildRecordOf(root, out, goadapter.Choice{})
	if outcome.Status != "built" {
		t.Fatalf("status %q, reason %q", outcome.Status, outcome.Reason)
	}
	if len(outcome.Artifacts) != 1 || outcome.Artifacts[0].Kind != "program" {
		t.Fatalf("artifacts: %+v", outcome.Artifacts)
	}
	if len(outcome.NotBuilt) != 1 || outcome.NotBuilt[0].Kind != "test program" {
		t.Fatalf("not built: %+v", outcome.NotBuilt)
	}
}

func TestARejectedTestDoesNotHideAnotherTestsLocalDependency(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"main.go":         "package main\n\nfunc main() {}\n",
		"main_test.go":    genericTest,
		"lib/lib.go":      library,
		"lib/lib_test.go": testsImportDep,
	})
	dep := filepath.Join(filepath.Dir(root), "dep")
	if err := os.Mkdir(dep, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dep, "go.mod"), "module example.com/dep\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(dep, "dep.go"), localDep)
	writeTestFile(t, filepath.Join(root, "go.mod"), sampleModule+"\nrequire example.com/dep v0.0.0\nreplace example.com/dep => ../dep\n")
	record := built(t, root, out, goadapter.Choice{})
	if len(record.NotBuilt) != 1 || len(record.LocalModules) != 1 || record.LocalModules[0].Path != "example.com/dep" {
		t.Fatalf("not built: %+v, local modules: %+v", record.NotBuilt, record.LocalModules)
	}
}
