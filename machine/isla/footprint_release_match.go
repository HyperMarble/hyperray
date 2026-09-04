// Release matching compares every manifest field with measured artifacts.
package isla

func (manifest releaseManifest) match(release FootprintRelease) error {
	comparisons := []struct {
		field    string
		expected string
		actual   string
	}{
		{field: "tool version", expected: manifest.ToolVersion, actual: release.tool.Version},
		{field: "tool digest", expected: manifest.ToolDigest, actual: release.tool.Digest},
		{field: "architecture digest", expected: manifest.ArchitectureDigest, actual: release.architecture.digest},
		{field: "configuration digest", expected: manifest.ConfigurationDigest, actual: release.configuration.digest},
	}
	for index := range comparisons {
		comparison := comparisons[index]
		if comparison.expected != comparison.actual {
			return releaseError(comparison.field + " differs")
		}
	}
	return nil
}
