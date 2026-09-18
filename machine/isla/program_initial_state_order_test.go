// Initial-state order must not change the declared program identity.
// Canonical output must leave the caller's assignments unchanged.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestProgramCanonicalizesTypedStateWithoutMutation(t *testing.T) {
	content := machineFixture(t)
	boundary := executableBoundary(0x80100000, "True")
	state := []isla.RegisterValue{{Name: "mode", Value: "Ready"}, {Name: "flag", Value: "true"}}
	boundary.InitialState = state
	first, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	boundary.InitialState = []isla.RegisterValue{state[1], state[0]}
	second, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() != second.Digest() || state[0].Name != "mode" {
		t.Fatal("state order changed the query identity or the caller's values")
	}
}
