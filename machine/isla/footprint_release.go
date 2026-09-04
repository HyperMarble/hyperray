// Footprint releases bind one tested engine and model artifact set.
// The binding is data, so Hyperray contains no release-specific digest.
package isla

// FootprintRelease is one identified Isla and Sail artifact set.
type FootprintRelease struct {
	id            string
	manifest      Artifact
	tool          ToolIdentity
	architecture  Artifact
	configuration Artifact
}

// NewFootprintRelease accepts a release only when every manifest value matches.
func NewFootprintRelease(manifest Artifact, engine FootprintEngine, architecture Artifact, configuration Artifact) (FootprintRelease, error) {
	values, err := readReleaseManifest(manifest.path)
	if err != nil {
		return FootprintRelease{}, err
	}
	release := FootprintRelease{
		id: values.ReleaseID, manifest: manifest, tool: engine.identity,
		architecture: architecture, configuration: configuration,
	}
	if err := release.current(); err != nil {
		return FootprintRelease{}, err
	}
	if err := values.match(release); err != nil {
		return FootprintRelease{}, err
	}
	return release, nil
}

func (release FootprintRelease) current() error {
	artifacts := []Artifact{release.manifest, release.architecture, release.configuration}
	for index := range artifacts {
		if err := artifacts[index].current(); err != nil {
			return err
		}
	}
	return toolCurrent(release.tool)
}

func (release FootprintRelease) matches(engine FootprintEngine) error {
	if release.tool != engine.identity {
		return releaseError("tool identity differs from request release")
	}
	return nil
}

func releaseError(detail string) error {
	return engineError(ReleaseMismatch, "footprint release", detail)
}
