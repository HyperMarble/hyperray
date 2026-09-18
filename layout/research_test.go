// This test keeps each C++ research source in its language directory.
// It does not classify manifests, solver input, or measured output as source.
package layout

import (
	"strings"
	"testing"
)

var cppResearchOwners = map[string]map[string]bool{
	".cpp": {"cpp": true},
	".py":  {"python": true},
}

func TestCppResearchSourceOwnership(t *testing.T) {
	failures, err := sourceOwnershipFailures(
		"../docs/cpp_adapter_artifacts",
		cppResearchOwners,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Errorf("source ownership errors:\n%s", strings.Join(failures, "\n"))
	}
}
