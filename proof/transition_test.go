// Transition tests require the full source-edge-target trace and state values.
// They never reduce an edge violation to a state-only claim.
package proof_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/model"
	"github.com/HyperMarble/hyperray/proof"
)

func TestDisprovedTransition(t *testing.T) {
	validated := modifiedFixture(t, addTransitionValues)
	query := proof.Query{
		RootIDs: []string{"main-root"},
		Requirement: proof.Requirement{
			ID: "no-compiled-step", Kind: proof.RequirementSafety,
			BadTransitionIDs: []string{"compiled-step"},
		},
	}
	result, err := proof.Check(validated, query)
	if err != nil {
		t.Fatalf("proof.Check() error = %v", err)
	}
	wantStates := []model.State{
		{ID: "start", Values: map[string]string{"balance": "32"}},
		{ID: "work", Values: map[string]string{"balance": "0"}},
	}
	if result.Witness == nil {
		t.Fatalf("proof.Check() witness = nil")
	}
	if result.Witness.Kind != proof.WitnessSafetyTransition ||
		result.Witness.TransitionID != "compiled-step" {
		t.Errorf("witness identity = %#v", result.Witness)
	}
	if !reflect.DeepEqual(result.Witness.Path.States, wantStates) ||
		!reflect.DeepEqual(result.Witness.Path.TransitionIDs, []string{"compiled-step"}) {
		t.Errorf("witness path = %#v", result.Witness.Path)
	}
}

func addTransitionValues(request *fixtureRequest) {
	values := map[string]map[string]string{
		"start": {"balance": "32"},
		"work":  {"balance": "0"},
	}
	for index := range request.Model.States {
		request.Model.States[index].Values = values[request.Model.States[index].ID]
	}
}
