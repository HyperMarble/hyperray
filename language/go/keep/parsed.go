// Purpose: parses the files a package is built from, each one the way the
// build reads it: the overlay's backing file when the overlay maps it.
// Never:   parses a file the overlay hides.
package keep

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// parsedFiles parses each file of the package that the overlay does not hide.
func parsedFiles(dir string, files []string, overlay map[string]string) ([]*ast.File, error) {
	set := token.NewFileSet()
	parsed := []*ast.File{}
	for _, file := range files {
		path, present := throughOverlay(filepath.Join(dir, file), overlay)
		if !present {
			continue
		}
		one, err := parser.ParseFile(set, path, nil, 0)
		if err != nil {
			return nil, record.Unreadable(file, err)
		}
		parsed = append(parsed, one)
	}
	return parsed, nil
}
