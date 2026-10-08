// Purpose: builds every program the module can make: each main package's
// executable, and each package's test program with every function
// kept, so a package's machine code is always in a file.
// Never:   writes into the module's folder; every output goes to out.
package build

import (
	"fmt"
	"github.com/HyperMarble/hyperray/language/go/keep"
	"path/filepath"

	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// built is one file the build made, before it is digested and read back.
type built struct {
	kind    string
	pkg     string
	path    string
	sources []record.FileDigest
}

const (
	programKind     = "program"
	testProgramKind = "test program"
)

func BuildAll(root, out string, choice record.Choice, packages []module.Package) ([]built, error) {
	flags, err := tool.BuildFlags(root, choice)
	if err != nil {
		return nil, err
	}
	theirs, _ := tool.FlagValue(flags, "overlay")
	overlay, err := keep.WriteOverlay(packages, out, theirs)
	if err != nil {
		return nil, err
	}
	files := []built{}
	for index, pkg := range packages {
		made, err := buildOne(root, out, choice, overlay, index, pkg)
		if err != nil {
			return nil, err
		}
		files = append(files, made...)
	}
	return files, nil
}

// buildOne makes the package's executable when it is a program with
// non-test files, and always its test program with the keep file added.
// `-vet=off`: go test runs vet by default, and vet's opinions (or its own
// crashes) must not stop a build that the compiler accepts.
func buildOne(root, out string, choice record.Choice, overlay string, index int, pkg module.Package) ([]built, error) {
	name := fmt.Sprintf("package_%d", index)
	files := []built{}
	if pkg.Name == "main" && len(pkg.GoFiles)+len(pkg.CgoFiles) > 0 {
		exe := filepath.Join(out, name)
		args := append([]string{"build"}, choice.Args()...)
		if _, err := tool.Printed(root, "go", append(args, "-o", exe, pkg.ImportPath)...); err != nil {
			return nil, err
		}
		sources, err := module.SourcesOf(pkg, false)
		if err != nil {
			return nil, err
		}
		files = append(files, built{programKind, pkg.ImportPath, exe, sources})
	}
	test := filepath.Join(out, name+".test")
	args := append([]string{"test"}, choice.Args()...)
	args = append(args, "-c", "-vet=off", "-overlay", overlay, "-o", test, pkg.ImportPath)
	if _, err := tool.Printed(root, "go", args...); err != nil {
		return nil, err
	}
	sources, err := module.SourcesOf(pkg, true)
	if err != nil {
		return nil, err
	}
	return append(files, built{testProgramKind, pkg.ImportPath, test, sources}), nil
}
