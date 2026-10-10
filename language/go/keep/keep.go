// Purpose: names every function and method in a package, its own test
// files, and its external test package, that Go code can refer to by
// value, so a generated test can keep each one linked with its own
// machine code (Go's linker drops what nothing reaches, and the compiler
// inlines small bodies away).
// Never:   names what Go cannot take by value: the init function, the blank
// name, generic functions and methods of generic types. Those have machine
// code only where they are used with concrete types, and that code is kept
// too.
package keep

import (
	"go/ast"

	"github.com/HyperMarble/hyperray/language/go/module"
)

// KeptNames lists the package's functions as Go expressions: `f`, `T.m`
// for a value receiver, `(*T).m` for a pointer receiver, from its Go and
// cgo files and its own test files, which the test program compiles too.
// Each file is read the way the build reads it: through the user's
// overlay when one maps it.
func KeptNames(pkg module.Package, overlay map[string]string) ([]string, error) {
	return namesInFiles(pkg.Dir, ownFiles(pkg), overlay)
}

// ownFiles are the files compiled into the package's own test program: its
// Go and cgo files and its own test files.
func ownFiles(pkg module.Package) []string {
	files := make([]string, 0, len(pkg.GoFiles)+len(pkg.CgoFiles)+len(pkg.TestGoFiles))
	files = append(files, pkg.GoFiles...)
	files = append(files, pkg.CgoFiles...)
	return append(files, pkg.TestGoFiles...)
}

// ExternalTestNames lists the functions of the package's external test
// package, the `_test` one Go compiles separately and links in.
func ExternalTestNames(pkg module.Package, overlay map[string]string) ([]string, error) {
	return namesInFiles(pkg.Dir, pkg.XTestGoFiles, overlay)
}

func namesInFiles(dir string, files []string, overlay map[string]string) ([]string, error) {
	parsed, err := parsedFiles(dir, files, overlay)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, file := range parsed {
		names = append(names, namesIn(file)...)
	}
	return names, nil
}

// throughOverlay is the file the build reads for path: the backing file
// when the overlay maps it, nothing when the overlay hides it.
func throughOverlay(path string, overlay map[string]string) (string, bool) {
	backing, mapped := overlay[path]
	if !mapped {
		return path, true
	}
	return backing, backing != ""
}

func namesIn(file *ast.File) []string {
	names := []string{}
	for _, decl := range file.Decls {
		if name, ok := keptName(decl); ok {
			names = append(names, name)
		}
	}
	return names
}

// keptName is the expression that refers to this declaration by value,
// or false when Go has no such expression for it.
func keptName(decl ast.Decl) (string, bool) {
	function, ok := decl.(*ast.FuncDecl)
	if !ok || function.Type.TypeParams != nil || !referable(function) {
		return "", false
	}
	if function.Recv == nil {
		return function.Name.Name, true
	}
	receiver, ok := receiverName(function.Recv.List[0].Type)
	return receiver + "." + function.Name.Name, ok
}

// referable is false for the two things the compiler refuses to refer to by
// value: the init function ("undefined: init") and the blank name ("cannot use
// _ as value"). Everything else has an expression, including a function named
// main and a method named init or main. A test asks the compiler.
func referable(function *ast.FuncDecl) bool {
	name := function.Name.Name
	return name != "_" && (name != "init" || function.Recv != nil)
}
