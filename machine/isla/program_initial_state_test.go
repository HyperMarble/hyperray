// Public state construction preserves typed data and its program identity.
// Conflicting declarations cannot produce a usable program.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestProgramPreservesTypedInitialState(t *testing.T) {
	content := machineFixture(t)
	boundary := executableBoundary(0x80100000, "True")
	first, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	boundary.InitialState = []isla.RegisterValue{{Name: "mtvec", Value: "{ bits = 0x000000008010000c }"}}
	second, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == second.Digest() || first.ImageDigest() != second.ImageDigest() {
		t.Fatal("typed initial state did not change only the query identity")
	}
	expected := "[initial_state]\nmtvec = \"{ bits = 0x000000008010000c }\"\n"
	if !strings.Contains(string(second.Content()), expected) {
		t.Fatalf("missing declared initial state: %s", second.Content())
	}
}

func TestProgramRejectsInvalidInitialState(t *testing.T) {
	content := machineFixture(t)
	cases := [][]isla.RegisterValue{
		{{Name: "x1", Value: "0x0000000000000000"}},
		{{Name: "flag", Value: "true"}, {Name: "flag", Value: "false"}},
		{{Name: "bad.name", Value: "true"}},
		{{Name: "flag", Value: ""}},
		{{Name: "flag", Value: "true\nfalse"}},
	}
	for _, state := range cases {
		boundary := executableBoundary(0x80100000, "True")
		boundary.InitialState = state
		program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
		assertProgramError(t, program, err, isla.InvalidInput)
	}
}

func TestProgramRejectsOversizedTypedState(t *testing.T) {
	content := machineFixture(t)
	boundary := executableBoundary(0x80100000, "True")
	baseline, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	boundary.MaximumProgramBytes = uint64(len(baseline.Content()))
	boundary.InitialState = []isla.RegisterValue{{Name: "flag", Value: "true"}}
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	assertProgramError(t, program, err, isla.ResourceLimit)
}
