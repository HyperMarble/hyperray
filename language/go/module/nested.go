// Purpose: finds every other go.mod under a module. One in a plain folder is
// a nested module, built as its own; one under a folder Go's own rules keep
// out of ./... (vendor, testdata, a name starting with "." or "_") is listed
// as ignored, by hash, and not built.
// Never:   lets code under an inner go.mod vanish from the record.
package module

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// ignoredName is the go tool's rule for a folder a pattern never enters.
func ignoredName(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// ignoredPath is true when any folder between root and dir is ignored.
func ignoredPath(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	for _, name := range strings.Split(rel, string(filepath.Separator)) {
		if ignoredName(name) {
			return true
		}
	}
	return false
}

// NestedModules lists the folders under root that hold their own go.mod:
// those to build, and those Go's rules ignore. A nested module's own
// nested modules are left to its build.
func NestedModules(root string) ([]string, []string, error) {
	toBuild, ignored := []string{}, []string{}
	err := filepath.WalkDir(root, func(dir string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || dir == root {
			return err
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
			return nil
		}
		if ignoredPath(root, dir) {
			ignored = append(ignored, dir)
			return nil
		}
		toBuild = append(toBuild, dir)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, nil, record.Unreadable(root, err)
	}
	return toBuild, ignored, nil
}
