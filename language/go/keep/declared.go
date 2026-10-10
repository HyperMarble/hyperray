// Purpose: lists every name a package's files declare at package level, so a
// generated file can take names the package does not have.
// Never:   counts a method, an init function or an import; none of them
// takes a name in the package.
package keep

import "go/ast"

// DeclaredNames is every package-level name the files declare, reading each
// file the way the build reads it: through the user's overlay.
func DeclaredNames(dir string, files []string, overlay map[string]string) (map[string]bool, error) {
	parsed, err := parsedFiles(dir, files, overlay)
	if err != nil {
		return nil, err
	}
	taken := map[string]bool{}
	for _, file := range parsed {
		for _, name := range declaredIn(file) {
			taken[name] = true
		}
	}
	return taken, nil
}

// declaredIn is every package-level name one file declares.
func declaredIn(file *ast.File) []string {
	names := []string{}
	for _, decl := range file.Decls {
		names = append(names, declaredBy(decl)...)
	}
	return names
}

// declaredBy is the package-level names one declaration brings in.
func declaredBy(decl ast.Decl) []string {
	switch typed := decl.(type) {
	case *ast.FuncDecl:
		if typed.Recv == nil && typed.Name.Name != "init" {
			return []string{typed.Name.Name}
		}
	case *ast.GenDecl:
		return specNames(typed.Specs)
	}
	return nil
}

func specNames(specs []ast.Spec) []string {
	names := []string{}
	for _, spec := range specs {
		names = append(names, namesOf(spec)...)
	}
	return names
}

// namesOf is the names one type, var or const line declares.
func namesOf(spec ast.Spec) []string {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		return []string{typed.Name.Name}
	case *ast.ValueSpec:
		return identNames(typed.Names)
	}
	return nil
}

func identNames(idents []*ast.Ident) []string {
	names := make([]string, 0, len(idents))
	for _, ident := range idents {
		names = append(names, ident.Name)
	}
	return names
}
