// Purpose: names every function and method in a package that Go code can
// refer to by value, so a generated test can keep each one linked
// with its own machine code (Go's linker drops what nothing reaches,
// and the compiler inlines small bodies away).
// Never:   names what Go cannot take by value: init, main, generic functions
// and methods of generic types. Those have machine code only where
// they are used with concrete types, and that code is kept too.
package keep

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"

	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
)

// KeptNames lists the package's functions as Go expressions: `f`, `T.m`
// for a value receiver, `(*T).m` for a pointer receiver. Each file is read
// the way the build reads it: through the user's overlay when one maps it.
func KeptNames(pkg module.Package, overlay map[string]string) ([]string, error) {
	set := token.NewFileSet()
	names := []string{}
	files := make([]string, 0, len(pkg.GoFiles)+len(pkg.CgoFiles))
	files = append(files, pkg.GoFiles...)
	files = append(files, pkg.CgoFiles...)
	for _, file := range files {
		path, present := throughOverlay(filepath.Join(pkg.Dir, file), overlay)
		if !present {
			continue
		}
		parsed, err := parser.ParseFile(set, path, nil, 0)
		if err != nil {
			return nil, record.Unreadable(file, err)
		}
		names = append(names, namesIn(parsed)...)
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
	if !ok || function.Type.TypeParams != nil || !referable(function.Name.Name) {
		return "", false
	}
	if function.Recv == nil {
		return function.Name.Name, true
	}
	receiver, ok := receiverName(function.Recv.List[0].Type)
	return receiver + "." + function.Name.Name, ok
}

func referable(name string) bool {
	return name != "init" && name != "main" && name != "_"
}
