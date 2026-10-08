// Purpose: names every function and method in a package that Go code can
// refer to by value, so a generated test can keep each one linked
// with its own machine code (Go's linker drops what nothing reaches,
// and the compiler inlines small bodies away).
// Never:   names what Go cannot take by value: init, main, generic functions
// and methods of generic types. Those have machine code only where
// they are used with concrete types, and that code is kept too.
package goadapter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
)

// keptNames lists the package's functions as Go expressions: `f`, `T.m`
// for a value receiver, `(*T).m` for a pointer receiver.
func keptNames(pkg Package) ([]string, error) {
	set := token.NewFileSet()
	names := []string{}
	for _, file := range pkg.GoFiles {
		parsed, err := parser.ParseFile(set, filepath.Join(pkg.Dir, file), nil, 0)
		if err != nil {
			return nil, unreadable(file, err)
		}
		names = append(names, namesIn(parsed)...)
	}
	return names, nil
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

// receiverName is `T` or `(*T)`; a generic receiver `T[X]` has no name
// that refers to one method body.
func receiverName(expr ast.Expr) (string, bool) {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name, true
	case *ast.StarExpr:
		ident, ok := typed.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		return "(*" + ident.Name + ")", true
	}
	return "", false
}
