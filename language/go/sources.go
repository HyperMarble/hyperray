// Purpose: names every source file a package's build compiles, by the hash
// of its bytes, so the record says exactly which bytes went in.
// Never:   hashes a file the build constraints excluded, or a derived list.
package goadapter

import "path/filepath"

// The `go list` fields whose files go into the package's program.
var programFileKinds = []string{
	"GoFiles", "CgoFiles", "CFiles", "CXXFiles", "MFiles", "HFiles", "FFiles",
	"SFiles", "SwigFiles", "SwigCXXFiles", "SysoFiles", "EmbedFiles",
}

// The fields whose files go only into the test program.
var testFileKinds = []string{"TestGoFiles", "TestEmbedFiles", "XTestGoFiles", "XTestEmbedFiles"}

// The fields that name files but are not inputs: a derived list, and the
// files the build constraints excluded.
var uncompiledFileKinds = []string{"CompiledGoFiles", "IgnoredGoFiles", "IgnoredOtherFiles"}

// fileLists is every file list `go list` reports for the package, by field.
func (pkg Package) fileLists() map[string][]string {
	return map[string][]string{
		"GoFiles": pkg.GoFiles, "CgoFiles": pkg.CgoFiles, "CFiles": pkg.CFiles,
		"CXXFiles": pkg.CXXFiles, "MFiles": pkg.MFiles, "HFiles": pkg.HFiles,
		"FFiles": pkg.FFiles, "SFiles": pkg.SFiles, "SwigFiles": pkg.SwigFiles,
		"SwigCXXFiles": pkg.SwigCXXFiles, "SysoFiles": pkg.SysoFiles,
		"EmbedFiles": pkg.EmbedFiles, "TestGoFiles": pkg.TestGoFiles,
		"TestEmbedFiles": pkg.TestEmbedFiles, "XTestGoFiles": pkg.XTestGoFiles,
		"XTestEmbedFiles": pkg.XTestEmbedFiles,
	}
}

// sourcesOf digests the files compiled into the package's program, or into
// its test program when tests is set, in the order `go list` reports them.
func sourcesOf(pkg Package, tests bool) ([]FileDigest, error) {
	kinds := programFileKinds
	if tests {
		kinds = append(append([]string{}, kinds...), testFileKinds...)
	}
	lists := pkg.fileLists()
	found := []FileDigest{}
	for _, kind := range kinds {
		digests, err := digestAll(pkg.Dir, lists[kind])
		if err != nil {
			return nil, err
		}
		found = append(found, digests...)
	}
	return found, nil
}

func digestAll(dir string, names []string) ([]FileDigest, error) {
	found := []FileDigest{}
	for _, name := range names {
		digest, err := digestOf(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		found = append(found, digest)
	}
	return found, nil
}
