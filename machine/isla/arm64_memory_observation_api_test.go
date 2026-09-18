// ARM64 observation API tests use the real checked-in Mach-O image.
// They must observe metadata without changing image bytes or execution semantics.
package isla_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramRetainsMemoryObservations(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	want := []isla.MemoryObservation{
		{Name: "lane1", Address: start + 4, Bytes: 8},
		{Name: "lane0", Address: start, Bytes: 4},
	}
	boundary := arm64ProgramBoundary(start, end)
	boundary.MemoryObservations = want
	program := buildObservedARM64Program(t, content, boundary)
	if got := program.MemoryObservations(); !reflect.DeepEqual(got, want) {
		t.Fatalf("MemoryObservations() = %#v, want %#v", got, want)
	}
	if got := program.Evidence().MemoryObservations; !reflect.DeepEqual(got, want) {
		t.Fatalf("Evidence().MemoryObservations = %#v, want %#v", got, want)
	}
}

func TestARM64MemoryObservationCopiesAreIsolated(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	boundary.MemoryObservations = []isla.MemoryObservation{{Name: "lane0", Address: start, Bytes: 4}}
	program := buildObservedARM64Program(t, content, boundary)
	boundary.MemoryObservations[0].Name = "callerChanged"
	observations := program.MemoryObservations()
	observations[0].Name = "accessorChanged"
	evidence := program.Evidence()
	evidence.MemoryObservations[0].Name = "evidenceChanged"
	if program.MemoryObservations()[0].Name != "lane0" || program.Evidence().MemoryObservations[0].Name != "lane0" {
		t.Fatal("observation metadata is not copy-isolated")
	}
}

func buildObservedARM64Program(t *testing.T, content []byte, boundary isla.ARM64ProgramBoundary) isla.Program {
	t.Helper()
	program, err := isla.BuildARM64Program(content, 32768, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	return program
}
