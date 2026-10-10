// Purpose: an overlay the project passes with a relative path, and relative
// paths written inside it, mean what they mean to Go: relative to the
// project folder, whichever folder the adapter was started from.
// Never:   reads the user's overlay from the adapter's own folder.
package goadapter_test

import (
	"os"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const replacedLibrary = library + "\nfunc onlyInReplacement(a int) int { return a * 5 }\n"

func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestARelativeOverlayFlagIsReadFromTheProjectFolder(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library, "overlay.json": `{"Replace":{}}`})
	built(t, root, out, goadapter.Choice{Flags: []string{"-overlay=overlay.json"}})
}

func TestRelativePathsInsideAnOverlayAreReadFromTheProjectFolder(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	writeText(t, filepath.Join(filepath.Dir(root), "replacement.go"), replacedLibrary)
	overlay := filepath.Join(root, "overlay.json")
	writeText(t, overlay, `{"Replace":{"lib/lib.go":"../replacement.go"}}`)
	record := built(t, root, out, goadapter.Choice{Flags: []string{"-overlay=" + overlay}})
	if len(record.Artifacts) == 0 {
		t.Fatalf("nothing was built: %+v", record.NotBuilt)
	}
	if !hasSymbol(symbols(t, record.Artifacts[0].File.Path), "lib.onlyInReplacement") {
		t.Fatal("a function that exists only in the overlay's replacement file has no machine code")
	}
}
