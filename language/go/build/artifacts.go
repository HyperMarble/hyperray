// Purpose: turns the built files into record entries: each one named by its
// hash and carrying Go's own build facts, and the packages that
// compile C, C++ or assembly.
// Never:   records a file without reading Go's build facts back out of it.
package build

import (
	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
)

func ArtifactsOf(files []built) ([]record.Artifact, error) {
	artifacts := []record.Artifact{}
	for _, file := range files {
		digest, err := record.DigestOf(file.path)
		if err != nil {
			return nil, err
		}
		info, err := ReadBuildInfo(file.path)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, record.Artifact{Kind: file.kind, Package: file.pkg, File: digest, Sources: file.sources, BuildInfo: info})
	}
	return artifacts, nil
}

func NativeCodeOf(packages []module.Package) []record.NativeCode {
	found := []record.NativeCode{}
	for _, pkg := range packages {
		if native := pkg.NativeCode(); native != nil {
			found = append(found, *native)
		}
	}
	return found
}
