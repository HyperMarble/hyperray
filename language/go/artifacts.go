// Purpose: turns the built files into record entries: each one named by its
// hash and carrying Go's own build facts, and the packages that
// compile C, C++ or assembly.
// Never:   records a file without reading Go's build facts back out of it.
package goadapter

func artifactsOf(files []built) ([]Artifact, error) {
	artifacts := []Artifact{}
	for _, file := range files {
		digest, err := digestOf(file.path)
		if err != nil {
			return nil, err
		}
		info, err := readBuildInfo(file.path)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{Kind: file.kind, Package: file.pkg, File: digest, BuildInfo: info})
	}
	return artifacts, nil
}

func nativeCodeOf(packages []Package) []NativeCode {
	found := []NativeCode{}
	for _, pkg := range packages {
		if native := pkg.nativeCode(); native != nil {
			found = append(found, *native)
		}
	}
	return found
}
