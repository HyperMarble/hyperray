// Purpose: a local folder that only the tests import is compiled into the
// test program, so it is hashed into the record like any other local folder.
// Never:   leaves code that went into a built file out of the record.
package goadapter_test

import (
	"os"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const testsImportDep = "package lib\n\nimport (\n\t\"testing\"\n\n\t\"example.com/dep\"\n)\n\nfunc TestV(t *testing.T) { _ = dep.V() }\n"

func TestALocalFolderOnlyTheTestsImportIsInTheRecord(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library, "lib/lib_test.go": testsImportDep})
	dep := filepath.Join(filepath.Dir(root), "dep")
	if err := os.Mkdir(dep, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dep, "go.mod"), "module example.com/dep\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(dep, "dep.go"), localDep)
	writeTestFile(t, filepath.Join(root, "go.mod"), sampleModule+"\nrequire example.com/dep v0.0.0\nreplace example.com/dep => ../dep\n")
	record := built(t, root, out, goadapter.Choice{})
	if len(record.LocalModules) != 1 || record.LocalModules[0].Path != "example.com/dep" {
		t.Fatalf("local modules: %+v", record.LocalModules)
	}
	sources := record.LocalModules[0].Sources
	if len(sources) != 1 || filepath.Base(sources[0].Path) != "dep.go" || sources[0].Sha256 == "" {
		t.Fatalf("sources: %+v", sources)
	}
}
