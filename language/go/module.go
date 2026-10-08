// Purpose: finds a Go module and the files that pin its versions.
// Never:   builds a folder that is not a module: without go.mod nothing says
// which Go version or which module versions to use.
package goadapter

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The files that pin a module's versions, when they exist.
var lockFileNames = []string{"go.mod", "go.sum"}

// lockFiles digests every version-pinning file the module has. go.mod
// must exist; the others exist only when the module has dependencies or
// is part of a workspace.
func lockFiles(root string) ([]FileDigest, error) {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return nil, noModule(root)
	}
	paths := []string{}
	for _, name := range lockFileNames {
		paths = append(paths, filepath.Join(root, name))
	}
	workspace, err := printed(root, "go", "env", "GOWORK")
	if err != nil {
		return nil, err
	}
	if work := strings.TrimSpace(workspace); work != "" && work != "off" {
		paths = append(paths, work, work+".sum")
	}
	found := []FileDigest{}
	for _, path := range paths {
		digest, present, err := digestIfPresent(path)
		if err != nil {
			return nil, err
		}
		if present {
			found = append(found, digest)
		}
	}
	return found, nil
}

// pinsUnchanged is an error when the pin files moved during the build.
func pinsUnchanged(root string, before []FileDigest) error {
	after, err := lockFiles(root)
	if err != nil {
		return err
	}
	if !slices.Equal(before, after) {
		return Blocked{Reason: "module pin files changed during build"}
	}
	return nil
}

func digestIfPresent(path string) (FileDigest, bool, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileDigest{}, false, nil
		}
		return FileDigest{}, false, unreadable(path, err)
	}
	digest, err := digestOf(path)
	return digest, err == nil, err
}
