// Safety tests measure exact fixed-point verdicts and state witnesses.
// They never accept a verdict without complete coverage.
package proof_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/model"
	"github.com/HyperMarble/hyperray/proof"
)

func TestProved(t *testing.T) {
	query := proof.Query{
		RootIDs: []string{"main-root"},
		Requirement: proof.Requirement{
			ID: "no-environment", Kind: proof.RequirementSafety,
			BadStateIDs: []string{"environment"},
		},
	}
	result, err := proof.Check(validatedFixture(t), query)
	if err != nil {
		t.Fatalf("proof.Check() error = %v", err)
	}
	if result.Verdict != proof.VerdictProved || result.Witness != nil {
		t.Fatalf("proof.Check() = %#v", result)
	}
	wantStates := []string{"start", "work"}
	wantTransitions := []string{"compiled-step", "synthetic-step"}
	if !reflect.DeepEqual(result.ReachableStateIDs, wantStates) ||
		!reflect.DeepEqual(result.ReachableTransitionIDs, wantTransitions) {
		t.Errorf("reachable = %v, %v", result.ReachableStateIDs, result.ReachableTransitionIDs)
	}
}

func TestDisprovedTrace(t *testing.T) {
	validated := modifiedFixture(t, addWitnessValues)
	query := proof.Query{
		RootIDs: []string{"main-root"},
		Requirement: proof.Requirement{
			ID: "no-work", Kind: proof.RequirementSafety,
			BadStateIDs: []string{"work"},
		},
	}
	result, err := proof.Check(validated, query)
	if err != nil {
		t.Fatalf("proof.Check() error = %v", err)
	}
	want := proof.Trace{States: []model.State{{ID: "work", Values: map[string]string{"phase": "active"}}}}
	if result.Verdict != proof.VerdictDisproved || result.Witness == nil {
		t.Fatalf("proof.Check() = %#v", result)
	}
	if result.Witness.RequirementID != "no-work" || result.Witness.StateID != "work" ||
		!reflect.DeepEqual(result.Witness.Path, want) {
		t.Errorf("witness = %#v, want path %#v", result.Witness, want)
	}
}

func addWitnessValues(request *fixtureRequest) {
	for index := range request.Model.States {
		if request.Model.States[index].ID == "work" {
			request.Model.States[index].Values = map[string]string{"phase": "active"}
		}
	}
}
