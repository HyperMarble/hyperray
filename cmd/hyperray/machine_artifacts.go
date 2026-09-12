// Artifact binding pins each tool input to a measured digest.
// It must never accept an input whose content it did not measure.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/HyperMarble/hyperray/machine/isla"
)

type toolArtifactSet struct {
	architecture  isla.Artifact
	configuration isla.Artifact
	memoryModel   isla.Artifact
	manifest      isla.Artifact
}

// toolArtifacts binds each pinned input to its measured digest. An artifact
// without a declared digest is measured from the file it names.
func toolArtifacts(tools machineTools) (toolArtifactSet, error) {
	architecture, err := pinnedArtifact(tools.Architecture, tools.ArchitectureID)
	if err != nil {
		return toolArtifactSet{}, fmt.Errorf("architecture: %w", err)
	}
	configuration, err := pinnedArtifact(tools.Configuration, "")
	if err != nil {
		return toolArtifactSet{}, fmt.Errorf("configuration: %w", err)
	}
	memoryModel, err := pinnedArtifact(tools.MemoryModel, "")
	if err != nil {
		return toolArtifactSet{}, fmt.Errorf("memory model: %w", err)
	}
	manifest, err := pinnedArtifact(tools.Manifest, tools.ManifestID)
	if err != nil {
		return toolArtifactSet{}, fmt.Errorf("manifest: %w", err)
	}
	return toolArtifactSet{architecture, configuration, memoryModel, manifest}, nil
}

// pinnedArtifact uses a declared digest when the caller supplies one, and
// otherwise measures the named file so the run records what it actually read.
func pinnedArtifact(path string, declaredDigest string) (isla.Artifact, error) {
	if declaredDigest != "" {
		return isla.NewArtifact(path, declaredDigest)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return isla.Artifact{}, err
	}
	measured := sha256.Sum256(content)
	return isla.NewArtifact(path, hex.EncodeToString(measured[:]))
}
