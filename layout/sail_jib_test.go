// This test keeps each Sail JIB tool source in its language directory.
// It does not classify the shell entry point as translator source.
package layout

import (
	"strings"
	"testing"
)

var sailJibOwners = map[string]map[string]bool{
	".go": {"go": true},
	".ml": {"ocaml": true},
}

func TestSailJibSourceOwnership(t *testing.T) {
	failures, err := sourceOwnershipFailures("../tools/sail-jib", sailJibOwners)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Errorf("source ownership errors:\n%s", strings.Join(failures, "\n"))
	}
}
