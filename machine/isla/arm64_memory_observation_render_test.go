// ARM64 observation rendering tests require canonical stable TOML output.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestARM64MemoryObservationsRenderByAddressThenName(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	first := []isla.MemoryObservation{
		{Name: "zeta", Address: start, Bytes: 4},
		{Name: "alpha", Address: start, Bytes: 2},
	}
	second := []isla.MemoryObservation{first[1], first[0]}
	one := arm64ProgramContent(t, content, start, end, first)
	two := arm64ProgramContent(t, content, start, end, second)
	if one != two {
		t.Fatal("rendered observations depend on declaration order")
	}
	for _, block := range []string{
		"[[memory_observations]]\nname = \"alpha\"\naddress = \"0x1000002e8\"\nbytes = 2",
		"[[memory_observations]]\nname = \"zeta\"\naddress = \"0x1000002e8\"\nbytes = 4",
	} {
		if !strings.Contains(one, block) {
			t.Errorf("generated ARM64 program lacks %q", block)
		}
	}
}

func arm64ProgramContent(t *testing.T, content []byte, start uint64, end uint64, observations []isla.MemoryObservation) string {
	t.Helper()
	boundary := arm64ProgramBoundary(start, end)
	boundary.MemoryObservations = observations
	return string(buildObservedARM64Program(t, content, boundary).Content())
}
