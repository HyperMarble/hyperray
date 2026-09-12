// The footprint release manifest is written from measured tool identity.
// The footprint release and the execution capability read different manifests.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/HyperMarble/hyperray/machine/isla"
)

// measuredFootprintRelease writes the release manifest the footprint engine
// requires, from the identity it reports and the pinned inputs it will read.
// The footprint release and the execution capability read different manifests.
func measuredFootprintRelease(footprints isla.FootprintEngine, architecture isla.Artifact,
	configuration isla.Artifact) (isla.FootprintRelease, error) {
	identity := footprints.Identity()
	manifest := map[string]string{
		"release_id":           "hyperray-machine",
		"tool_version":         identity.Version,
		"tool_sha256":          identity.Digest,
		"architecture_sha256":  architecture.Digest(),
		"configuration_sha256": configuration.Digest(),
	}
	content, err := json.Marshal(manifest)
	if err != nil {
		return isla.FootprintRelease{}, fmt.Errorf("release manifest: %w", err)
	}
	artifact, err := writeMeasuredArtifact(content, "footprint-release.json")
	if err != nil {
		return isla.FootprintRelease{}, fmt.Errorf("release manifest: %w", err)
	}
	release, err := isla.NewFootprintRelease(artifact, footprints, architecture, configuration)
	if err != nil {
		return isla.FootprintRelease{}, fmt.Errorf("footprint release: %w", err)
	}
	return release, nil
}

func writeMeasuredArtifact(content []byte, name string) (isla.Artifact, error) {
	directory, err := os.MkdirTemp("", "hyperray-release")
	if err != nil {
		return isla.Artifact{}, err
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return isla.Artifact{}, err
	}
	digest := sha256.Sum256(content)
	return isla.NewArtifact(path, hex.EncodeToString(digest[:]))
}
