// Release manifests use strict JSON and complete identity fields.
package isla

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
)

type releaseManifest struct {
	ReleaseID           string `json:"release_id"`
	ToolVersion         string `json:"tool_version"`
	ToolDigest          string `json:"tool_sha256"`
	ArchitectureDigest  string `json:"architecture_sha256"`
	ConfigurationDigest string `json:"configuration_sha256"`
}

func readReleaseManifest(path string) (releaseManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return releaseManifest{}, releaseError(err.Error())
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var manifest releaseManifest
	if err := decoder.Decode(&manifest); err != nil {
		return releaseManifest{}, releaseError(err.Error())
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return releaseManifest{}, releaseError("manifest has trailing JSON")
	}
	if err := manifest.valid(); err != nil {
		return releaseManifest{}, err
	}
	return manifest, nil
}

func (manifest releaseManifest) valid() error {
	if manifest.ReleaseID == "" || manifest.ToolVersion == "" {
		return releaseError("manifest has an empty identity")
	}
	digests := []string{manifest.ToolDigest, manifest.ArchitectureDigest, manifest.ConfigurationDigest}
	for index := range digests {
		if !validDigest(digests[index]) {
			return releaseError("manifest has an invalid SHA-256 digest")
		}
	}
	return nil
}
