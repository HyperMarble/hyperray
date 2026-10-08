// Purpose: every folder holding Go code is accounted for in exactly one
// place: a built file's sources, a nested module, or an ignored module.
// Never:   lets a folder with Go code belong to no part of the record.
package goadapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

func TestEveryFolderWithGoCodeIsAccountedFor(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"lib/lib.go": library, "inner/go.mod": innerModule, "inner/x/x.go": innerSource,
		"testdata/m/go.mod": innerModule, "testdata/m/m.go": innerSource,
	})
	record := built(t, root, out, goadapter.Choice{})
	placed := placedDirs(record)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if !placed[filepath.Dir(path)] {
			t.Errorf("%s belongs to no part of the record", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// placedDirs is every folder the record accounts for: a source's folder,
// a nested module's tree, or an ignored module's tree.
func placedDirs(record *goadapter.BuildRecord) map[string]bool {
	dirs := map[string]bool{}
	for _, artifact := range record.Artifacts {
		for _, source := range artifact.Sources {
			dirs[filepath.Dir(source.Path)] = true
		}
	}
	for _, nested := range record.Nested {
		for dir := range placedDirs(nested.Record) {
			dirs[dir] = true
		}
	}
	for _, ignored := range record.IgnoredModules {
		markTree(dirs, filepath.Dir(ignored.Path))
	}
	return dirs
}

func markTree(dirs map[string]bool, root string) {
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			dirs[path] = true
		}
		return err
	})
}
