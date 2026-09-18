// Artifact identity tests reject empty and duplicate artifact catalogs.
// They never select a duplicate's digest by input order.
package coverage_test

import "testing"

func TestArtifactIdentityFailures(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Artifacts[0].ID = ""
	requireCoverageError(t, request, "empty_id")
	request = completeRequest(t)
	request.Inventory.Artifacts = append(request.Inventory.Artifacts,
		request.Inventory.Artifacts[0])
	requireCoverageError(t, request, "duplicate_id")
}
