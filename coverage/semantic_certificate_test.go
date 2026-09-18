// Semantic certificate tests reject duplicate, missing, and wrong-slice rows.
// They never infer semantic claims from independent bindings.
package coverage_test

import "testing"

func TestSemanticCertificateDuplicate(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Synthetic = append(request.Certificate.Synthetic,
		request.Certificate.Synthetic[0])
	requireCoverageError(t, request, "duplicate_evidence")
}

func TestSyntheticCertificateMissing(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Synthetic = nil
	requireCoverageError(t, request, "missing_semantic_mapping")
}

func TestEnvironmentCertificateMissing(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Environment = nil
	requireCoverageError(t, request, "missing_semantic_mapping")
}

func TestSemanticCertificateWrongKind(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Synthetic[0].OperationID = "environment"
	requireCoverageError(t, request, "wrong_mapping_kind")
}
