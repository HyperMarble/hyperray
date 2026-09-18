// Typed state requires support from both real program stages.
// Existing queries and isolated footprints retain their earlier contract.
package isla

import (
	"slices"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestTypedInitialStateRequiresBothCapabilities(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		query := Request{typedInitialState: enabled}
		request := VerificationRequest{query: query}
		if slices.Contains(query.arguments(), "--typed-initial-state") != enabled {
			t.Errorf("solver capability differs: %v", query.arguments())
		}
		if slices.Contains(request.semanticArguments(), "--typed-initial-state") != enabled {
			t.Errorf("semantic capability differs: %v", request.semanticArguments())
		}
	}
}

func TestFootprintExcludesTypedInitialState(t *testing.T) {
	request := FootprintRequest{}
	if slices.Contains(request.arguments(machine.Instruction{}), "--typed-initial-state") {
		t.Fatal("isolated footprint selects program initial state")
	}
}
