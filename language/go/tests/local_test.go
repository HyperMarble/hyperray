// Purpose: a dependency replaced by a folder, and a module a go.work use
// line brings in, are hashed into the record by their compiled sources;
// the root module itself is never listed as local.
// Never:   lets code from a local folder go unpinned.
package goadapter_test

import (
	"os"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const localDep = "package dep\n\nfunc V() int { return 1 }\n"
const usesLocalDep = "package lib\n\nimport \"example.com/dep\"\n\nfunc L() int { return dep.V() }\n"

func TestADirectoryReplaceIsHashed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": usesLocalDep})
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

func TestAWorkspaceSiblingIsHashedAndTheRootIsNot(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": usesLocalDep})
	base := filepath.Dir(root)
	if err := os.Mkdir(filepath.Join(base, "dep"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(base, "dep", "go.mod"), "module example.com/dep\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(base, "dep", "dep.go"), localDep)
	writeTestFile(t, filepath.Join(root, "go.mod"), sampleModule+"\nrequire example.com/dep v0.0.0\n")
	writeTestFile(t, filepath.Join(base, "go.work"), "go 1.25\n\nuse (\n\t./module\n\t./dep\n)\n")
	record := built(t, root, out, goadapter.Choice{})
	if len(record.LocalModules) != 1 || record.LocalModules[0].Path != "example.com/dep" {
		t.Fatalf("local modules: %+v", record.LocalModules)
	}
	if _, err := os.Stat(record.LocalModules[0].Dir); err != nil {
		t.Fatal(err)
	}
}
