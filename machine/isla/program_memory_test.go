// The public memory profile changes the artifact, not the source program.
// Unsupported profile names must return an error before proof execution.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestProgramMemoryProfileIsExplicit(t *testing.T) {
	content := machineFixture(t)
	image, err := machine.Load(content, uint64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	boundary := executableBoundary(image.EntryAddress, "~(0:x11 = 7)")
	legacy, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	boundary.MemoryProfile = isla.SequentialMemory
	sequential, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sequential.Content()), "memory_profile = \"sequential\"") {
		t.Fatal("generated artifact lacks the explicit memory contract")
	}
	if strings.Contains(string(legacy.Content()), "memory_profile") || sequential.Digest() == legacy.Digest() {
		t.Error("memory contract is absent from artifact identity")
	}
	boundary.MemoryProfile = "unknown"
	if _, err := isla.BuildProgram(content, uint64(len(content)), boundary); err == nil {
		t.Error("unknown memory profile was accepted")
	}
}
