// ARM64 observation defaults preserve the existing no-observation artifact.
// An absent declaration must not create metadata or memory effects.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramOmitsAbsentMemoryObservations(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	program, err := isla.BuildARM64Program(content, 32768, arm64ProgramBoundary(start, end))
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	if program.MemoryObservations() != nil || program.Evidence().MemoryObservations != nil {
		t.Fatal("absent observations are not represented as absent metadata")
	}
	if strings.Contains(string(program.Content()), "memory_observations") {
		t.Fatal("absent observations changed generated artifact")
	}
}
