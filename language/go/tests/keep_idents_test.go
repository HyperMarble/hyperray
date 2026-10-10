// Purpose: a package that already declares the names the keep file uses
// still gets its test program, with every function kept; the keep file
// takes names the package does not have.
// Never:   lets the keep file's names clash with the package's own.
package goadapter_test

import (
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const libraryWithKeepName = library + "\nvar hyperrayKeep = 1\n"

const ownKeepTest = "package lib\n\nimport \"testing\"\n\nfunc TestHyperrayKeep(t *testing.T) {}\n"

func TestAPackageThatAlreadyUsesTheKeepNamesStillGetsItsTestProgram(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"lib/lib.go": libraryWithKeepName, "lib/own_test.go": ownKeepTest,
	})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.NotBuilt) > 0 {
		t.Fatalf("not built: %+v", record.NotBuilt)
	}
	names := symbols(t, record.Artifacts[0].File.Path)
	for _, want := range []string{"lib.unused", "lib.TestHyperrayKeep", "lib.TestHyperrayKeep1"} {
		if !hasSymbol(names, want) {
			t.Fatalf("%s is missing from the test program", want)
		}
	}
}
