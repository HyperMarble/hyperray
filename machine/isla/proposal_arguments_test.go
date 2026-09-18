// The output extension is enabled only for its declared tool version.
// Unpatched engines retain the legacy invocation.
package isla

import (
	"slices"
	"testing"
)

func TestCandidateStatusArguments(t *testing.T) {
	legacy := proposalArguments(ToolIdentity{Version: "v0.2.0/test"}, Request{})
	patched := proposalArguments(ToolIdentity{Version: "v0.2.0/test/candidate-status-v1"}, Request{})
	if slices.Contains(legacy, "--candidate-status") || !slices.Contains(patched, "--candidate-status") {
		t.Errorf("legacy = %v, patched = %v", legacy, patched)
	}
	extended := proposalArguments(ToolIdentity{Version: "v0.2.0/test/candidate-status-v1/executable-entry-v1"}, Request{})
	if !slices.Contains(extended, "--candidate-status") {
		t.Errorf("extended release lost candidate labels: %v", extended)
	}
}
