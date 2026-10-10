// Purpose: writes one generated keep file: a test file that refers to every
// named function by value, under a name the package does not already use.
// Never:   uses a predeclared name in the file, or hides a file the package
// or the overlay already has.
package keep

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
)

// The name the generated file has inside the package, for the overlay. An
// overlay makes the build see our file at that path, so the name must be
// one the package does not already use, or its own file would vanish.
const keepFileName = "hyperray_keep_test.go"

// freeName is keepFileName when neither the package nor the overlay has a
// file by that name, else the first numbered variant neither has.
func freeName(dir string, taken map[string]string) string {
	name := keepFileName
	for count := 1; exists(filepath.Join(dir, name)) || taken[filepath.Join(dir, name)] != ""; count++ {
		name = fmt.Sprintf("hyperray_keep_%d_test.go", count)
	}
	return name
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// The file uses no predeclared name (`any`, `len`, `nil`): a package may
// redefine any of them, and Go's own suite has one that redefines them all.
// Its own names (the import, the list, the test) are ones the package does
// not declare: see idents.go.
const keepFileText = `package %[1]s

import %[2]s

var %[3]s = []interface{}{%[4]s}

func %[5]s(t *%[6]s.T) {
	t.Log(%[3]s)
}
`

// keepFilesFor writes the package's keep file, and its external test
// package's when it has one, and maps each into the package folder.
func keepFilesFor(pkg module.Package, user, replace map[string]string, out string, index int) error {
	names, err := KeptNames(pkg, user)
	if err != nil {
		return err
	}
	taken, err := DeclaredNames(pkg.Dir, ownFiles(pkg), user)
	if err != nil {
		return err
	}
	path := filepath.Join(out, fmt.Sprintf("keep_%d_test.go", index))
	if err := writeKeepFile(pkg.Name, names, identsFor(taken), path); err != nil {
		return err
	}
	replace[filepath.Join(pkg.Dir, freeName(pkg.Dir, replace))] = path
	if len(pkg.XTestGoFiles) == 0 {
		return nil
	}
	return keepExternalFile(pkg, user, replace, out, index)
}

// keepExternalFile writes the keep file of the package's external test package.
func keepExternalFile(pkg module.Package, user, replace map[string]string, out string, index int) error {
	external, err := ExternalTestNames(pkg, user)
	if err != nil {
		return err
	}
	taken, err := DeclaredNames(pkg.Dir, pkg.XTestGoFiles, user)
	if err != nil {
		return err
	}
	path := filepath.Join(out, fmt.Sprintf("keep_%d_external_test.go", index))
	if err := writeKeepFile(pkg.Name+"_test", external, identsFor(taken), path); err != nil {
		return err
	}
	replace[filepath.Join(pkg.Dir, freeName(pkg.Dir, replace))] = path
	return nil
}

func writeKeepFile(packageName string, names []string, idents keepIdents, path string) error {
	text := fmt.Sprintf(keepFileText, packageName, idents.importLine(), idents.variable,
		strings.Join(names, ", "), idents.test, idents.testing)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return record.Unreadable(path, err)
	}
	return nil
}
