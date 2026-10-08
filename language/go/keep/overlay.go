// Purpose: writes the generated test file that keeps every function linked,
// and the overlay that puts it into each package for the build.
// Never:   writes into the module's folder, or hides a file the package
// already has: the overlay only ever adds a name the package does not use.
package keep

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// The name the generated file has inside the package, for the overlay. An
// overlay makes the build see our file at that path, so the name must be
// one the package does not already use, or its own file would vanish.
const keepFileName = "hyperray_keep_test.go"

// freeName is keepFileName when the package has no file by that name, else
// the first numbered variant it does not have.
func freeName(dir string) string {
	name := keepFileName
	for count := 1; exists(filepath.Join(dir, name)); count++ {
		name = fmt.Sprintf("hyperray_keep_%d_test.go", count)
	}
	return name
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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
// path of the overlay that maps each into its package. Go takes one
// overlay, so a user's overlay, when there is one, is merged in first.
func WriteOverlay(packages []module.Package, out, userOverlay string) (string, error) {
	replace := map[string]string{}
	if userOverlay != "" {
		theirs, err := tool.ReadOverlay(userOverlay)
		if err != nil {
			return "", err
		}
		replace = theirs
	}
	user := map[string]string{}
	for disk, backing := range replace {
		user[disk] = backing
	}
	for index, pkg := range packages {
		path, err := writeKeepFile(pkg, user, filepath.Join(out, fmt.Sprintf("keep_%d_test.go", index)))
		if err != nil {
			return "", err
		}
		replace[filepath.Join(pkg.Dir, freeName(pkg.Dir))] = path
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

func writeKeepFile(pkg module.Package, overlay map[string]string, path string) (string, error) {
	names, err := KeptNames(pkg, overlay)
	if err != nil {
		return "", err
	}
	text := fmt.Sprintf(keepFileText, pkg.Name, strings.Join(names, ", "))
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", record.Unreadable(path, err)
	}
	return path, nil
}
