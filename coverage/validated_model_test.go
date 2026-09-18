// Validated-model tests observe the exact canonical model stored by the token.
// They never let a caller replace the graph after coverage succeeds.
package coverage_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestValidatedCoverageCanonicalModel(t *testing.T) {
	request := completeRequest(t)
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	graph, err := validated.Model()
	if err != nil {
		t.Fatalf("Model() error = %v", err)
	}
	stateIDs := []string{graph.States[0].ID, graph.States[1].ID, graph.States[2].ID}
	transitionIDs := []string{
		graph.Transitions[0].ID, graph.Transitions[1].ID, graph.Transitions[2].ID,
	}
	if !reflect.DeepEqual(stateIDs, []string{"environment", "start", "work"}) {
		t.Errorf("state IDs = %v", stateIDs)
	}
	if !reflect.DeepEqual(transitionIDs,
		[]string{"compiled-step", "environment-step", "synthetic-step"}) {
		t.Errorf("transition IDs = %v", transitionIDs)
	}
}

func TestValidatedCoverageModelDeepCopy(t *testing.T) {
	request := completeRequest(t)
	for index := range request.Model.States {
		request.Model.States[index].Values = map[string]string{"phase": "before"}
	}
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	first, err := validated.Model()
	if err != nil {
		t.Fatalf("Model() error = %v", err)
	}
	first.States[0].Values["phase"] = "after"
	second, err := validated.Model()
	if err != nil || second.States[0].Values["phase"] != "before" {
		t.Errorf("Model() copy = %#v, error = %v", second, err)
	}
}
