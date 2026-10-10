// Purpose: lists every folder with Go code that ./... left out, with the
// reason go list gives for it and the files it left out by hash. Go's own
// case: build constraints exclude every file on this machine.
// Never:   decides for itself why a folder was left out; it asks go list.
package module

import (
	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// listed is what `go list -e` reports for one folder on its own.
type listed struct {
	Dir               string
	IgnoredGoFiles    []string
	IgnoredOtherFiles []string
	Error             *struct{ Err string }
}

// ExcludedFolders asks go list about every folder holding Go code that is
// neither a listed package, a nested module, nor a folder Go's rules skip.
func ExcludedFolders(root string, choice record.Choice, packages []Package) ([]record.Excluded, error) {
	known := map[string]bool{}
	for _, pkg := range packages {
		known[pkg.Dir] = true
	}
	nested, _, err := NestedModules(root)
	if err != nil {
		return nil, err
	}
	user, err := tool.UserOverlay(root, choice)
	if err != nil {
		return nil, err
	}
	found := []record.Excluded{}
	for _, dir := range goFolders(root, nested) {
		if known[dir] {
			continue
		}
		excluded, err := excludedFolder(root, dir, choice, user)
		if err != nil {
			return nil, err
		}
		if excluded != nil {
			found = append(found, *excluded)
		}
	}
	return found, nil
}

// excludedFolder is go list's account of one folder, or nothing when go
// list has no complaint about it.
func excludedFolder(root, dir string, choice record.Choice, user map[string]string) (*record.Excluded, error) {
	args := append([]string{"list", "-e", "-json=Dir,IgnoredGoFiles,IgnoredOtherFiles,Error"}, choice.Args()...)
	text, err := tool.Printed(root, "go", append(args, dir)...)
	if err != nil {
		return nil, err
	}
	var entry listed
	if err := decodeOne(text, &entry); err != nil {
		return nil, err
	}
	if entry.Error == nil {
		return nil, nil
	}
	files, err := digestAll(dir, append(append([]string{}, entry.IgnoredGoFiles...), entry.IgnoredOtherFiles...), user)
	if err != nil {
		return nil, err
	}
	return &record.Excluded{Dir: dir, Reason: entry.Error.Err, Files: files}, nil
}
