// Purpose: a package that already has a file named hyperray_keep_test.go
// keeps it: the overlay adds the keep file under a free name instead of
// replacing the real one.
// Never:   lets the overlay hide a file the package already has.
package goadapter_test

import (
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const realKeepNamedFile = "package lib\n\nimport \"testing\"\n\nfunc TestReal(t *testing.T) { _ = unused(1) }\n"

func TestAPackageOwnFileNamedLikeTheKeepFileSurvives(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"lib/lib.go": library, "lib/hyperray_keep_test.go": realKeepNamedFile,
	})
	record := built(t, root, out, goadapter.Choice{})
	names := symbols(t, record.Artifacts[0].File.Path)
	for _, want := range []string{"lib.TestReal", "lib.TestHyperrayKeep", "lib.unused"} {
		if !hasSymbol(names, want) {
			t.Fatalf("%s is missing from the test program", want)
		}
	}
}
