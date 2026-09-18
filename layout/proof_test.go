// This test keeps each proof source language in its named directory.
// It must not inspect generated proof output.
package layout

import (
	"strings"
	"testing"
)

var proofSourceOwners = map[string]map[string]bool{
	".lean": {"lean": true},
	".sail": {"sail": true},
	".sh":   {"shell": true},
}

func TestIntegerProofSourceOwnership(t *testing.T) {
	failures, err := sourceOwnershipFailures("../proof/sail_addi", proofSourceOwners)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Errorf("source ownership errors:\n%s", strings.Join(failures, "\n"))
	}
}
