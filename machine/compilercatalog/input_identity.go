// Referenced input identities are collected without allowing path collisions.
// A caller must provide bytes for every identity named by the manifest.
package compilercatalog

import "fmt"

func expectedInputs(build BuildManifest) (map[string]ArtifactIdentity, error) {
	inputs := make(map[string]ArtifactIdentity)
	if err := addExpectedInput(inputs, build.Object); err != nil {
		return nil, err
	}
	for _, identity := range build.ExternArtifacts {
		if err := addExpectedInput(inputs, identity); err != nil {
			return nil, err
		}
	}
	if build.BoundaryArtifact != nil {
		if err := addExpectedInput(inputs, *build.BoundaryArtifact); err != nil {
			return nil, err
		}
	}
	return inputs, nil
}

func addExpectedInput(inputs map[string]ArtifactIdentity, identity ArtifactIdentity) error {
	if identity.Path == "" {
		return fmt.Errorf("empty referenced artifact path")
	}
	if _, exists := inputs[identity.Path]; exists {
		return fmt.Errorf("duplicate referenced artifact: %s", identity.Path)
	}
	inputs[identity.Path] = identity
	return nil
}

func inputName(identity ArtifactIdentity, build BuildManifest) string {
	if identity.Path == build.Object.Path {
		return "object"
	}
	return "referenced artifact"
}

func validateArtifactBytes(content []byte, expected ArtifactIdentity, name string) error {
	if err := sizeMatches(content, expected.Size, name); err != nil {
		return err
	}
	return hashMatches(content, expected.SHA256, name)
}
