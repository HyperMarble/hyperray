// Purpose: hashes the code of every module the build takes from a folder on
// this machine rather than from the module cache: a `replace` that points at
// a directory, or a module a go.work `use` line brings in. go.sum pins
// neither, so the record pins them here.
// Never:   lists the root module itself, or a module go.sum already pins.
package goadapter

import "path/filepath"

// ModuleInfo is what `go list` reports about a package's module.
type ModuleInfo struct {
	Path    string
	Main    bool
	Dir     string
	Replace *ModuleInfo
}

// LocalModule is one locally sourced module and the sources compiled from it.
type LocalModule struct {
	Path    string       `json:"path"`
	Dir     string       `json:"dir"`
	Sources []FileDigest `json:"sources"`
}

// isLocal is true for a module whose code comes from a folder go.sum does
// not pin: a directory replace, or a workspace module other than root's.
func (module *ModuleInfo) isLocal(root string) bool {
	if module == nil {
		return false
	}
	if module.Replace != nil && module.Replace.Dir != "" {
		return true
	}
	return module.Main && otherFolder(module.Dir, root)
}

// otherFolder is true when the two paths name different folders, links followed.
func otherFolder(a, b string) bool {
	realA, errA := filepath.EvalSymlinks(a)
	realB, errB := filepath.EvalSymlinks(b)
	return errA != nil || errB != nil || realA != realB
}

// localModules walks every package the build depends on and hashes the
// sources of those that come from a local module, grouped by module.
func localModules(root string, choice Choice) ([]LocalModule, error) {
	args := append([]string{"list", "-deps", listedFields}, choice.args()...)
	text, err := printed(root, "go", append(args, "./...")...)
	if err != nil {
		return nil, err
	}
	packages, err := decodePackages(text)
	if err != nil {
		return nil, err
	}
	return groupLocal(packages, root)
}

func groupLocal(packages []Package, root string) ([]LocalModule, error) {
	found := []LocalModule{}
	at := map[string]int{}
	for _, pkg := range packages {
		if !pkg.Module.isLocal(root) {
			continue
		}
		sources, err := sourcesOf(pkg, false)
		if err != nil {
			return nil, err
		}
		index, seen := at[pkg.Module.Path]
		if !seen {
			index = len(found)
			at[pkg.Module.Path] = index
			found = append(found, LocalModule{Path: pkg.Module.Path, Dir: pkg.Module.Dir})
		}
		found[index].Sources = append(found[index].Sources, sources...)
	}
	return found, nil
}
