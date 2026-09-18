// Input closure validation compares each supplied artifact with the manifest.
// Missing source, runtime, and sysroot closure remains an explicit obligation.
package compilercatalog

import (
	"fmt"
	"sort"
)

func validateObjectInputs(objects []ObjectArtifact, build BuildManifest) error {
	expected, err := expectedInputs(build)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(objects))
	for _, object := range objects {
		if _, exists := seen[object.Path]; exists {
			return fmt.Errorf("duplicate object artifact: %s", object.Path)
		}
		seen[object.Path] = struct{}{}
		identity, exists := expected[object.Path]
		if !exists {
			return fmt.Errorf("extra object artifact: %s", object.Path)
		}
		if err := validateArtifactBytes(object.Content, identity, inputName(identity, build)); err != nil {
			return err
		}
	}
	paths := make([]string, 0, len(expected))
	for path := range expected {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if _, exists := seen[path]; exists {
			continue
		}
		if path == build.Object.Path {
			return fmt.Errorf("missing object artifact: %s", path)
		}
		return fmt.Errorf("missing referenced artifact: %s", path)
	}
	return nil
}
