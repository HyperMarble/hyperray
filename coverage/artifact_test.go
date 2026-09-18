// Artifact tests reject malformed digests and stale digest-bound references.
// They never resolve evidence through an identifier alone.
package coverage_test

import "testing"

func TestBadArtifactDigest(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Artifacts[0].SHA256 = "BAD"
	requireCoverageError(t, request, "invalid_sha256")
}

func TestStaleArtifact(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.CompilerOutputs[0].Artifact.SHA256 =
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	requireCoverageError(t, request, "stale_artifact")
}

func TestUnknownArtifact(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.CompilerOutputs[0].Artifact.ArtifactID = "artifact:stale"
	requireCoverageError(t, request, "unknown_reference")
}

func TestUppercaseArtifactDigest(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Artifacts[0].SHA256 =
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	requireCoverageError(t, request, "invalid_sha256")
}

func TestArtifactContentDigestMismatch(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Artifacts[0].Content = []byte("fabricated")
	requireCoverageError(t, request, "artifact_digest_mismatch")
}

func TestEmptyArtifactContent(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Artifacts[0].Content = nil
	requireCoverageError(t, request, "empty_field")
}

func TestEmptyArtifactReference(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.CompilerOutputs[0].Artifact.ArtifactID = ""
	requireCoverageError(t, request, "empty_field")
}
