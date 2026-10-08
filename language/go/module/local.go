// Purpose: hashes the code of every module the build takes from a folder on
// this machine rather than from the module cache: a `replace` that points at
// a directory, or a module a go.work `use` line brings in. go.sum pins
// neither, so the record pins them here.
// Never:   lists the root module itself, or a module go.sum already pins.
package module

import (
	"path/filepath"

	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// ModuleInfo is what `go list` reports about a package's module.
type ModuleInfo struct {
	Path    string
	Main    bool
	Dir     string
	Replace *ModuleInfo
}

// isLocal is true for a module whose code comes from a folder go.sum does
// not pin: a directory replace, or a workspace module other than root's.
func (info *ModuleInfo) isLocal(root string) bool {
	if info == nil {
		return false
	}
	if info.Replace != nil && info.Replace.Dir != "" {
		return true
	}
	return info.Main && otherFolder(info.Dir, root)
}

// otherFolder is true when the two paths name different folders, links followed.
func otherFolder(a, b string) bool {
	realA, errA := filepath.EvalSymlinks(a)
	realB, errB := filepath.EvalSymlinks(b)
	return errA != nil || errB != nil || realA != realB
}

// LocalModules walks every package the build depends on and hashes the
// sources of those that come from a local module, grouped by module.
func LocalModules(root string, choice record.Choice) ([]record.LocalModule, error) {
	args := append([]string{"list", "-deps", listedFields}, choice.Args()...)
	text, err := tool.Printed(root, "go", append(args, "./...")...)
	if err != nil {
		return nil, err
	}
	packages, err := DecodePackages(text)
	if err != nil {
		return nil, err
	}
	return groupLocal(packages, root)
}

func groupLocal(packages []Package, root string) ([]record.LocalModule, error) {
	found := []record.LocalModule{}
	at := map[string]int{}
	for _, pkg := range packages {
		if !pkg.Module.isLocal(root) {
			continue
		}
		sources, err := SourcesOf(pkg, false)
		if err != nil {
			return nil, err
		}
		index, seen := at[pkg.Module.Path]
		if !seen {
			index = len(found)
			at[pkg.Module.Path] = index
			found = append(found, record.LocalModule{Path: pkg.Module.Path, Dir: pkg.Module.Dir})
		}
		found[index].Sources = append(found[index].Sources, sources...)
	}
	return found, nil
}
