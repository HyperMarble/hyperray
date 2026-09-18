// This test keeps source files in the adapter that owns their language.
// It must not require an adapter before that adapter has real source code.
package layout

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var sourceOwners = map[string]map[string]bool{
	".c":  {"c": true},
	".cc": {"cpp": true}, ".cpp": {"cpp": true},
	".cxx": {"cpp": true}, ".hxx": {"cpp": true},
	".go": {"go": true},
	".h":  {"c": true, "cpp": true},
	".hh": {"cpp": true}, ".hpp": {"cpp": true},
	".ixx": {"cpp": true},
	".ml":  {"ocaml": true},
	".py":  {"python": true},
	".rs":  {"rust": true},
}

func TestAdapterSourceOwnership(t *testing.T) {
	failures, err := sourceOwnershipFailures("../adapters", sourceOwners)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Errorf("source ownership errors:\n%s", strings.Join(failures, "\n"))
	}
}

func TestSailCatalogSourceOwnership(t *testing.T) {
	failures, err := sourceOwnershipFailures("../tools/sail-catalog", sourceOwners)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Errorf("source ownership errors:\n%s", strings.Join(failures, "\n"))
	}
}

func sourceOwnershipFailures(root string, owners map[string]map[string]bool) ([]string, error) {
	var failures []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, readError error) error {
		if readError != nil {
			return readError
		}
		if entry.IsDir() || owners[filepath.Ext(path)] == nil {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		adapter := strings.Split(relative, string(filepath.Separator))[0]
		if owners[filepath.Ext(path)][adapter] {
			return nil
		}
		failures = append(failures, relative)
		return nil
	})
	sort.Strings(failures)
	return failures, err
}
