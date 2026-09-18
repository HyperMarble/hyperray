// ARM64 namespace tests distinguish exact native names from padded spellings.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestARM64MemoryObservationsAllowUnconfiguredPaddedNames(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	boundary.MemoryObservations = []isla.MemoryObservation{
		{Name: "R00", Address: start, Bytes: 1},
		{Name: "X00", Address: start + 1, Bytes: 1},
		{Name: "W00", Address: start + 2, Bytes: 1},
		{Name: "SP_EL00", Address: start + 3, Bytes: 1},
	}
	if _, err := isla.BuildARM64Program(content, 32768, boundary); err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
}
