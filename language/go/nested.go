// Purpose: finds every other go.mod under a module. One in a plain folder is
// a nested module, built as its own; one under a folder Go's own rules keep
// out of ./... (vendor, testdata, a name starting with "." or "_") is listed
// as ignored, by hash, and not built.
// Never:   lets code under an inner go.mod vanish from the record.
package goadapter

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Nested is a module inside another, built as its own.
type Nested struct {
	Dir    string       `json:"dir"`
	Record *BuildRecord `json:"record"`
}

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

// nestedModules lists the folders under root that hold their own go.mod:
// those to build, and those Go's rules ignore. A nested module's own
// nested modules are left to its build.
func nestedModules(root string) ([]string, []string, error) {
	build, ignored := []string{}, []string{}
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
		build = append(build, dir)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, nil, unreadable(root, err)
	}
	return build, ignored, nil
}

// withNested builds every nested module into its own folder under out and
// hashes the go.mod of every ignored one.
func withNested(root, out string, choice Choice, record *BuildRecord) (*BuildRecord, error) {
	build, ignored, err := nestedModules(root)
	if err != nil {
		return nil, err
	}
	for index, dir := range build {
		nestedOut := filepath.Join(out, fmt.Sprintf("nested_%d", index))
		if err := os.MkdirAll(nestedOut, 0o755); err != nil {
			return nil, unreadable(nestedOut, err)
		}
		nested, err := tryBuildRecord(dir, nestedOut, choice)
		if err != nil {
			return nil, err
		}
		record.Nested = append(record.Nested, Nested{Dir: dir, Record: nested})
	}
	for _, dir := range ignored {
		digest, err := digestOf(filepath.Join(dir, "go.mod"))
		if err != nil {
			return nil, err
		}
		record.IgnoredModules = append(record.IgnoredModules, digest)
	}
	return record, nil
}
