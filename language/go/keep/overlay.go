// Purpose: writes the generated test file that keeps every function linked,
// and the overlay that puts it into each package for the build.
// Never:   writes into the module's folder: Go's `-overlay` adds the file
// to the build only, and the file itself lives in the output folder.
package keep

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
)

// The name the generated file has inside the package, for the overlay.
const keepFileName = "hyperray_keep_test.go"

// The file uses no predeclared name (`any`, `len`, `nil`): a package may
// redefine any of them, and Go's own suite has one that redefines them all.
const keepFileText = `package %s

import "testing"

var hyperrayKeep = []interface{}{%s}

func TestHyperrayKeep(t *testing.T) {
	t.Log(hyperrayKeep)
}
`

// WriteOverlay writes one keep file per package into out and returns the
// path of the overlay that maps each into its package.
func WriteOverlay(packages []module.Package, out string) (string, error) {
	replace := map[string]string{}
	for index, pkg := range packages {
		path, err := writeKeepFile(pkg, filepath.Join(out, fmt.Sprintf("keep_%d_test.go", index)))
		if err != nil {
			return "", err
		}
		replace[filepath.Join(pkg.Dir, keepFileName)] = path
	}
	text, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		return "", record.Unreadable("overlay", err)
	}
	overlay := filepath.Join(out, "overlay.json")
	if err := os.WriteFile(overlay, text, 0o644); err != nil {
		return "", record.Unreadable(overlay, err)
	}
	return overlay, nil
}

func writeKeepFile(pkg module.Package, path string) (string, error) {
	names, err := KeptNames(pkg)
	if err != nil {
		return "", err
	}
	text := fmt.Sprintf(keepFileText, pkg.Name, strings.Join(names, ", "))
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", record.Unreadable(path, err)
	}
	return path, nil
}
