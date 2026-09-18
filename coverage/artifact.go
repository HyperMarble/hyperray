// Artifact validation binds every artifact identity to a lowercase SHA-256.
// It never accepts a name without the exact digest.
package coverage

import (
	"crypto/sha256"
	"encoding/hex"
)

const sha256HexLength = 64

func collectArtifacts(catalog *catalogs, inventory CompilerInventory) error {
	ids := make([]string, 0, len(inventory.Artifacts))
	for _, artifact := range inventory.Artifacts {
		ids = append(ids, artifact.ID)
		catalog.artifacts[artifact.ID] = artifact
	}
	var err error
	if catalog.artifactIDs, err = uniqueIDSet(ids, "artifact"); err != nil {
		return err
	}
	for _, id := range sortedArtifactIDs(catalog.artifacts) {
		artifact := catalog.artifacts[id]
		if !validSHA256(artifact.SHA256) {
			return coverageError("invalid_sha256", "artifact", id)
		}
		if len(artifact.Content) == 0 {
			return coverageError("empty_field", "artifact.content", id)
		}
		digest := sha256.Sum256(artifact.Content)
		if hex.EncodeToString(digest[:]) != artifact.SHA256 {
			return coverageError("artifact_digest_mismatch", id)
		}
	}
	return nil
}

func requireArtifact(catalog *catalogs, reference ArtifactReference, field string) error {
	if err := requireEvidence(
		evidenceField{field + ".artifact_id", reference.ArtifactID},
		evidenceField{field + ".sha256", reference.SHA256},
	); err != nil {
		return err
	}
	if err := requireReference(catalog.artifactIDs, reference.ArtifactID, "artifact"); err != nil {
		return err
	}
	artifact := catalog.artifacts[reference.ArtifactID]
	if artifact.SHA256 != reference.SHA256 {
		return coverageError("stale_artifact", reference.ArtifactID, reference.SHA256)
	}
	catalog.used.artifacts[reference.ArtifactID] = struct{}{}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256HexLength {
		return false
	}
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9':
		case character >= 'a' && character <= 'f':
		default:
			return false
		}
	}
	return true
}
