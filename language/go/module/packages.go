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
// build's own tags and flags, so the file lists match what gets built. A
// module where ./... matches nothing is a module with nothing to build,
// which is what go build itself does with it, not a refusal.
func ListPackages(root string, choice record.Choice) ([]Package, error) {
	args := append([]string{"list", listedFields}, choice.Args()...)
	text, err := tool.Printed(root, "go", append(args, "./...")...)
	if err != nil {
		return nil, err
	}
	return DecodePackages(text)
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

// NativeCode names the package's files that are not Go source, if any.
func (pkg Package) NativeCode() *record.NativeCode {
	lists, total := pkg.fileLists(), 0
	for _, kind := range nativeFileKinds {
		total += len(lists[kind])
	}
	if total == 0 {
		return nil
	}
	return &record.NativeCode{
		Package: pkg.ImportPath, CgoFiles: pkg.CgoFiles, CFiles: pkg.CFiles, CxxFiles: pkg.CXXFiles,
		ObjectiveCFiles: pkg.MFiles, HeaderFiles: pkg.HFiles, FortranFiles: pkg.FFiles,
		AssemblyFiles: pkg.SFiles, SwigFiles: pkg.SwigFiles, SwigCxxFiles: pkg.SwigCXXFiles,
		ObjectFiles: pkg.SysoFiles,
	}
}

// decodeOne reads the single JSON object `go list -e -json <dir>` prints.
func decodeOne(text string, into any) error {
	if err := json.NewDecoder(strings.NewReader(text)).Decode(into); err != nil {
		return record.Unreadable("go list output", err)
	}
	return nil
}
