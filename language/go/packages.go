// Purpose: lists the module's own packages as the go tool sees them for this
// build: the files the tags select, and any C or assembly among them.
// Never:   lists a dependency's package as the module's own.
package goadapter

import (
	"encoding/json"
	"strings"
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

// listPackages asks the go tool for every package under root, with the
// build's own tags and flags, so the file lists match what gets built.
func listPackages(root string, choice Choice) ([]Package, error) {
	args := append([]string{"list", listedFields}, choice.args()...)
	text, err := printed(root, "go", append(args, "./...")...)
	if err != nil {
		return nil, err
	}
	found, err := decodePackages(text)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, noPackages(root)
	}
	return found, nil
}

// decodePackages reads the stream of packages `go list -json` prints.
func decodePackages(text string) ([]Package, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	found := []Package{}
	for decoder.More() {
		var pkg Package
		if err := decoder.Decode(&pkg); err != nil {
			return nil, unreadable("go list output", err)
		}
		found = append(found, pkg)
	}
	return found, nil
}

// nativeCode names the package's C, C++ and assembly files, if it has any.
func (pkg Package) nativeCode() *NativeCode {
	if len(pkg.CgoFiles)+len(pkg.CFiles)+len(pkg.CXXFiles)+len(pkg.SFiles) == 0 {
		return nil
	}
	return &NativeCode{
		Package:       pkg.ImportPath,
		CgoFiles:      pkg.CgoFiles,
		CFiles:        pkg.CFiles,
		CxxFiles:      pkg.CXXFiles,
		AssemblyFiles: pkg.SFiles,
	}
}
