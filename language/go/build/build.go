// Purpose: builds every program the module can make: each main package's
// executable, and each package's test program with every function
// kept, so a package's machine code is always in a file. A test program
// the project's own flags forbid is recorded as not built, with Go's words.
// Never:   writes into the module's folder; every output goes to out.
package build

import (
	"fmt"
	"github.com/HyperMarble/hyperray/language/go/keep"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// built is one file the build made, before it is digested and read back.
type built struct {
	kind    string
	pkg     string
	path    string
	mode    string
	sources []record.FileDigest
}

const (
	programKind     = "program"
	testProgramKind = "test program"
)

func BuildAll(root, out string, choice record.Choice, packages []module.Package) ([]built, []record.NotBuilt, error) {
	flags, err := tool.BuildFlags(root, choice)
	if err != nil {
		return nil, nil, err
	}
	theirs, _ := tool.FlagValue(flags, "overlay")
	overlay, err := keep.WriteOverlay(packages, out, theirs)
	if err != nil {
		return nil, nil, err
	}
	mode, _ := tool.FlagValue(flags, "buildmode")
	files, notBuilt := []built{}, []record.NotBuilt{}
	for index, pkg := range packages {
		made, missing, err := buildOne(root, out, choice, overlay, mode, index, pkg)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, made...)
		notBuilt = append(notBuilt, missing...)
	}
	return files, notBuilt, nil
}

// buildOne makes the package's executable when it is a program with
// non-test files, and always its test program with the keep file added.
// `-vet=off`: go test runs vet by default, and vet's opinions (or its own
// crashes) must not stop a build that the compiler accepts.
// `-buildmode=default`: a test program is an executable, as `go test` makes
// it; a requested buildmode applies to the program, not to it.
func buildOne(root, out string, choice record.Choice, overlay, mode string, index int, pkg module.Package) ([]built, []record.NotBuilt, error) {
	name := fmt.Sprintf("%d_%s", index, strings.ReplaceAll(pkg.ImportPath, "/", "_"))
	files := []built{}
	if pkg.Name == "main" && len(pkg.GoFiles)+len(pkg.CgoFiles) > 0 {
		exe := filepath.Join(out, name)
		args := append([]string{"build"}, choice.Args()...)
		if _, err := tool.Printed(root, "go", append(args, "-o", exe, pkg.ImportPath)...); err != nil {
			return nil, nil, err
		}
		sources, err := module.SourcesOf(pkg, false)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, built{programKind, pkg.ImportPath, exe, mode, sources})
	}
	test := filepath.Join(out, name+".test")
	args := append([]string{"test"}, choice.Args()...)
	args = append(args, "-c", "-vet=off", "-buildmode=default", "-overlay", overlay, "-o", test, pkg.ImportPath)
	if _, err := tool.Printed(root, "go", args...); err != nil {
		// The project's own build made the program; a flag it builds with
		// can still forbid linking a test program. That is recorded, not refused.
		return files, []record.NotBuilt{{Kind: testProgramKind, Package: pkg.ImportPath, Reason: err.Error()}}, nil
	}
	sources, err := module.SourcesOf(pkg, true)
	if err != nil {
		return nil, nil, err
	}
	return append(files, built{testProgramKind, pkg.ImportPath, test, "", sources}), nil, nil
}
