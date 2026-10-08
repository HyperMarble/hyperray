// Purpose: builds every program the module can make: each main package's
// executable, and each package's test program with every function
// kept, so a package's machine code is always in a file.
// Never:   writes into the module's folder; every output goes to out.
package goadapter

import (
	"path/filepath"
	"strings"
)

// built is one file the build made, before it is digested and read back.
type built struct {
	kind string
	pkg  string
	path string
}

const (
	programKind     = "program"
	testProgramKind = "test program"
)

func buildAll(root, out string, choice Choice, packages []Package) ([]built, error) {
	overlay, err := writeOverlay(packages, out)
	if err != nil {
		return nil, err
	}
	files := []built{}
	for _, pkg := range packages {
		made, err := buildOne(root, out, choice, overlay, pkg)
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
func buildOne(root, out string, choice Choice, overlay string, pkg Package) ([]built, error) {
	name := strings.ReplaceAll(pkg.ImportPath, "/", "_")
	files := []built{}
	if pkg.Name == "main" && len(pkg.GoFiles)+len(pkg.CgoFiles) > 0 {
		exe := filepath.Join(out, name)
		args := append([]string{"build"}, choice.args()...)
		if _, err := printed(root, "go", append(args, "-o", exe, pkg.ImportPath)...); err != nil {
			return nil, err
		}
		files = append(files, built{programKind, pkg.ImportPath, exe})
	}
	test := filepath.Join(out, name+".test")
	args := append([]string{"test"}, choice.args()...)
	args = append(args, "-c", "-vet=off", "-overlay", overlay, "-o", test, pkg.ImportPath)
	if _, err := printed(root, "go", args...); err != nil {
		return nil, err
	}
	return append(files, built{testProgramKind, pkg.ImportPath, test}), nil
}
