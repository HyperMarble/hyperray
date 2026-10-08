// Purpose: finds a Go module and the files that pin its versions.
// Never:   builds a folder that is not a module: without go.mod nothing says
// which Go version or which module versions to use.
package goadapter

import (
	"os"
	"path/filepath"
)

// The files that pin a module's versions, when they exist.
var lockFileNames = []string{"go.mod", "go.sum", "go.work", "go.work.sum"}

// lockFiles digests every version-pinning file the module has. go.mod
// must exist; the others exist only when the module has dependencies or
// is part of a workspace.
func lockFiles(root string) ([]FileDigest, error) {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return nil, noModule(root)
	}
	found := []FileDigest{}
	for _, name := range lockFileNames {
		digest, present, err := digestIfPresent(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		if present {
			found = append(found, digest)
		}
	}
	return found, nil
}

func digestIfPresent(path string) (FileDigest, bool, error) {
	if _, err := os.Stat(path); err != nil {
		return FileDigest{}, false, nil
	}
	digest, err := digestOf(path)
	return digest, err == nil, err
}
