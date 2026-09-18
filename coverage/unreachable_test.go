// Unreachable-operation tests retain disconnected operations and transitions.
// They never inspect reachability when computing semantic coverage.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
	"github.com/HyperMarble/hyperray/model"
)

func TestUnreachableOperation(t *testing.T) {
	request := completeRequest(t)
	request.Model.States = append(request.Model.States, model.State{ID: "disconnected"})
	request.Model.Transitions = append(request.Model.Transitions, model.Transition{
		ID: "disconnected-step", FromStateID: "disconnected", ToStateID: "disconnected",
	})
	request.Inventory.Operations = append(request.Inventory.Operations, coverage.Operation{
		ID: "unreachable", Kind: coverage.OperationCompiler,
		Location: "dead.go:1", Disposition: coverage.DispositionMapped,
	})
	addUnreachableEdges(&request)
	addUnreachableBinding(&request)
	report, err := checkRequest(request)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if report.MappedOperations != 4 || report.MappedTransitions != 4 {
		t.Errorf("unreachable counts = %d, %d", report.MappedOperations, report.MappedTransitions)
	}
}
