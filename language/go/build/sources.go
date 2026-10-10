// Purpose: hashes the source files each built file was made from, the way
// the build read them: through the user's overlay.
// Never:   hashes a file from disk when the overlay replaced or added it.
package build

import "github.com/HyperMarble/hyperray/language/go/module"

// withSources hashes the source files each built file was made from, through
// the user's overlay: the bytes the build read.
func withSources(made []built, pkg module.Package, user map[string]string) ([]built, error) {
	for index := range made {
		sources, err := module.SourcesOf(pkg, made[index].kind == testProgramKind, user)
		if err != nil {
			return nil, err
		}
		made[index].sources = sources
	}
	return made, nil
}
