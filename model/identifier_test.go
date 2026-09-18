// These tests apply the schema identifier pattern through the public API.
// They never extend that pattern to arbitrary state values.
package model_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestIdentifierAcceptsEverySchemaCharacterClass(t *testing.T) {
	identifier := "9A.z_:/-"
	graph := model.Model{
		States:      []model.State{{ID: identifier, Values: map[string]string{"display name": "exact"}}},
		Transitions: []model.Transition{{ID: identifier, FromStateID: identifier, ToStateID: identifier}},
	}
	if err := model.Validate(graph); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestIdentifierRejectsMalformedStateID(t *testing.T) {
	graph := model.Model{States: []model.State{{ID: "-bad"}}}
	requireValidation(t, graph, "invalid_identifier", "-bad", "states.id")
}

func TestIdentifierRejectsNonASCIIStateID(t *testing.T) {
	graph := model.Model{States: []model.State{{ID: "é"}}}
	requireValidation(t, graph, "invalid_identifier", "states.id", "é")
}

func TestIdentifierRejectsMalformedTransitionID(t *testing.T) {
	edge := model.Transition{ID: "bad id", FromStateID: "ready", ToStateID: "ready"}
	graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: []model.Transition{edge}}
	requireValidation(t, graph, "invalid_identifier", "bad id", "transitions.id")
}

func TestIdentifierRejectsMalformedFromStateID(t *testing.T) {
	edge := model.Transition{ID: "step", FromStateID: "bad id", ToStateID: "ready"}
	graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: []model.Transition{edge}}
	requireValidation(t, graph, "invalid_identifier", "bad id", "step", "transitions.from_state_id")
}

func TestIdentifierRejectsMalformedToStateID(t *testing.T) {
	edge := model.Transition{ID: "step", FromStateID: "ready", ToStateID: "bad id"}
	graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: []model.Transition{edge}}
	requireValidation(t, graph, "invalid_identifier", "bad id", "step", "transitions.to_state_id")
}

func TestIdentifierRejectsEmptyEndpoint(t *testing.T) {
	edge := model.Transition{ID: "step", ToStateID: "ready"}
	graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: []model.Transition{edge}}
	requireValidation(t, graph, "invalid_identifier", "step", "transitions.from_state_id")
}
