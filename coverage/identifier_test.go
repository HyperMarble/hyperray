// Identifier tests apply the schema alphabet to declarations and references.
// They never apply it to a source location.
package coverage_test

import "testing"

func TestMalformedIdentifier(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Functions[0].ID = "bad id"
	requireCoverageError(t, request, "invalid_identifier")
}

func TestMalformedReference(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Machine[0].TransitionID = "bad#id"
	requireCoverageError(t, request, "invalid_identifier")
}

func TestIdentifierMustStartAlphanumeric(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Functions[0].ID = "-bad"
	requireCoverageError(t, request, "invalid_identifier")
}

func TestEmptyReference(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Roots[0].FunctionID = ""
	requireCoverageError(t, request, "empty_id")
}

func TestLocationIsPlainText(t *testing.T) {
	request := completeRequest(t)
	request.Inventory.Operations[0].Location = "source file.go line 10 # exact"
	if _, err := checkRequest(request); err != nil {
		t.Errorf("Check() error = %v", err)
	}
}
