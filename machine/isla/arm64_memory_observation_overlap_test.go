// ARM64 observation overlap tests cover independent metadata ranges.
// Overlap must remain valid because observations have no execution effects.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramAcceptsOverlappingMemoryObservations(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	boundary.MemoryObservations = []isla.MemoryObservation{
		{Name: "word", Address: start, Bytes: 8},
		{Name: "tail", Address: start + 4, Bytes: 4},
	}
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	if len(program.MemoryObservations()) != 2 {
		t.Fatalf("MemoryObservations() length = %d, want 2", len(program.MemoryObservations()))
	}
}
