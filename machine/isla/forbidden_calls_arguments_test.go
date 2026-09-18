// Both whole-program stages require instrumented model-call support.
// Isolated instruction analysis must not inherit program safety conditions.
package isla

import (
	"slices"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestForbiddenCallsRequireBothCapabilities(t *testing.T) {
	query := Request{forbiddenModelCalls: []string{"trap_handler"}}
	request := VerificationRequest{query: query}
	expected := []string{"--forbidden-model-calls", "--trace-function", "trap_handler"}
	for _, arguments := range [][]string{query.arguments(), request.semanticArguments()} {
		if !slices.Equal(arguments[len(arguments)-len(expected):], expected) {
			t.Errorf("call instrumentation differs: %v", arguments)
		}
	}
	footprint := FootprintRequest{}
	if slices.Contains(footprint.arguments(machine.Instruction{}), "--forbidden-model-calls") {
		t.Fatal("isolated footprint selected a program condition")
	}
}
