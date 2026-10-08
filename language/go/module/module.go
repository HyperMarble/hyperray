// Purpose: finds a Go module and the files that pin its versions.
// Never:   builds a folder that is not a module: without go.mod nothing says
// which Go version or which module versions to use.
package module

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// The files that pin a module's versions, when they exist.
var lockFileNames = []string{"go.mod", "go.sum"}

// LockFiles digests every version-pinning file the module has. go.mod
// must exist; the others exist only when the module has dependencies or
// is part of a workspace.
func LockFiles(root string) ([]record.FileDigest, error) {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return nil, record.NoModule(root)
	}
	paths := []string{}
	for _, name := range lockFileNames {
		paths = append(paths, filepath.Join(root, name))
	}
	workspace, err := tool.Printed(root, "go", "env", "GOWORK")
	if err != nil {
		return nil, err
	}
	if work := strings.TrimSpace(workspace); work != "" && work != "off" {
		paths = append(paths, work, work+".sum")
	}
	found := []record.FileDigest{}
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

// PinsAfter is the pin files after the build, and whether any moved.
func PinsAfter(root string, before []record.FileDigest) ([]record.FileDigest, bool, error) {
	after, err := LockFiles(root)
	if err != nil {
		return nil, false, err
	}
	return after, !slices.Equal(before, after), nil
}

func digestIfPresent(path string) (record.FileDigest, bool, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return record.FileDigest{}, false, nil
		}
		return record.FileDigest{}, false, record.Unreadable(path, err)
	}
	digest, err := record.DigestOf(path)
	return digest, err == nil, err
}
