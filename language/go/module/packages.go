// Purpose: lists the module's own packages as the go tool sees them for this
// build: the files the tags select, and any C or assembly among them.
// Never:   lists a dependency's package as the module's own.
package module

import (
	"encoding/json"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// Package is what `go list` reports about one package of the module.
type Package struct {
	ImportPath      string
	Name            string
	Dir             string
	GoFiles         []string
	CgoFiles        []string
	CFiles          []string
	CXXFiles        []string
	SFiles          []string
	MFiles          []string
	HFiles          []string
	FFiles          []string
	SwigFiles       []string
	SwigCXXFiles    []string
	SysoFiles       []string
	EmbedFiles      []string
	TestGoFiles     []string
	TestEmbedFiles  []string
	XTestGoFiles    []string
	XTestEmbedFiles []string
	Module          *ModuleInfo
}

const listedFields = "-json=ImportPath,Name,Dir,GoFiles,CgoFiles,CFiles,CXXFiles,SFiles," +
	"MFiles,HFiles,FFiles,SwigFiles,SwigCXXFiles,SysoFiles,EmbedFiles," +
	"TestGoFiles,TestEmbedFiles,XTestGoFiles,XTestEmbedFiles,Module"

// ListPackages asks the go tool for every package under root, with the
// build's own tags and flags, so the file lists match what gets built.
func ListPackages(root string, choice record.Choice) ([]Package, error) {
	args := append([]string{"list", listedFields}, choice.Args()...)
	text, err := tool.Printed(root, "go", append(args, "./...")...)
	if err != nil {
		return nil, err
	}
	found, err := DecodePackages(text)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, record.NoPackages(root)
	}
	return found, nil
}

// DecodePackages reads the stream of packages `go list -json` prints.
func DecodePackages(text string) ([]Package, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	found := []Package{}
	for decoder.More() {
		var pkg Package
		if err := decoder.Decode(&pkg); err != nil {
			return nil, record.Unreadable("go list output", err)
		}
		found = append(found, pkg)
	}
	return found, nil
}

// NativeCode names the package's C, C++ and assembly files, if it has any.
func (pkg Package) NativeCode() *record.NativeCode {
	if len(pkg.CgoFiles)+len(pkg.CFiles)+len(pkg.CXXFiles)+len(pkg.SFiles) == 0 {
		return nil
	}
	return &record.NativeCode{
		Package:       pkg.ImportPath,
		CgoFiles:      pkg.CgoFiles,
		CFiles:        pkg.CFiles,
		CxxFiles:      pkg.CXXFiles,
		AssemblyFiles: pkg.SFiles,
	}
}
