// Root certificate tests reject unknown, duplicate, and missing claims.
// They never use certificate order as evidence.
package coverage_test

import "testing"

func TestRootCertificateIdentityFailures(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Roots = append(request.Certificate.Roots, request.Certificate.Roots[0])
	requireCoverageError(t, request, "duplicate_id")
	request = completeRequest(t)
	request.Certificate.Roots = request.Certificate.Roots[:1]
	requireCoverageError(t, request, "missing_root_mapping")
	request = completeRequest(t)
	request.Certificate.Roots[0].RootID = "root:missing"
	requireCoverageError(t, request, "unknown_reference")
}

func TestRootCertificateReferenceFailures(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Roots[0].StateIDs[0] = "state:missing"
	requireCoverageError(t, request, "unknown_reference")
	request = completeRequest(t)
	request.Certificate.Roots[1].ImpossiblePreconditionProofID = "proof:missing"
	requireCoverageError(t, request, "unknown_reference")
	request = completeRequest(t)
	request.Certificate.Roots[0].ProvenanceEdgeIDs[0] = "edge:missing"
	requireCoverageError(t, request, "unknown_reference")
}

func TestDuplicateRootCertificateState(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Roots[0].StateIDs = []string{"start", "start"}
	requireCoverageError(t, request, "duplicate_id")
}
