// Public call requirements bind identity without modifying caller input.
// Invalid declarations must never produce a usable program.
package isla_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestProgramBindsForbiddenCalls(t *testing.T) {
	content := machineFixture(t)
	boundary := executableBoundary(0x80100000, "True")
	boundary.MemoryProfile = isla.SequentialMemory
	baseline, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	boundary.ForbiddenModelCalls = []string{"trap_handler", "handle_mem_exception"}
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Digest() == program.Digest() || baseline.ImageDigest() != program.ImageDigest() {
		t.Fatal("call requirement did not change only query identity")
	}
	if !slices.Equal(boundary.ForbiddenModelCalls, []string{"trap_handler", "handle_mem_exception"}) {
		t.Fatal("caller input changed")
	}
	expected := "forbidden_model_calls = [\"handle_mem_exception\", \"trap_handler\"]\n"
	if !strings.Contains(string(program.Content()), expected) {
		t.Fatalf("missing sorted call requirement: %s", program.Content())
	}
	boundary.MaximumProgramBytes = uint64(len(baseline.Content()))
	program, err = isla.BuildProgram(content, uint64(len(content)), boundary)
	assertProgramError(t, program, err, isla.ResourceLimit)
}

func TestProgramRejectsInvalidForbiddenCalls(t *testing.T) {
	content := machineFixture(t)
	for _, names := range [][]string{{""}, {"bad.name"}, {"same", "same"}} {
		boundary := executableBoundary(0x80100000, "True")
		boundary.MemoryProfile = isla.SequentialMemory
		boundary.ForbiddenModelCalls = names
		program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
		assertProgramError(t, program, err, isla.InvalidInput)
	}
	boundary := executableBoundary(0x80100000, "True")
	boundary.ForbiddenModelCalls = []string{"trap_handler"}
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	assertProgramError(t, program, err, isla.InvalidInput)
}
