// These tests prove that public validation ignores graph slice order.
// They never inspect the package's internal sorting representation.
package model_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestStateValidationOrderIsInvariant(t *testing.T) {
	emptyName := map[string]string{"": "bad"}
	first := model.Model{States: []model.State{{ID: "z", Values: emptyName}, {ID: "a", Values: emptyName}}}
	second := model.Model{States: []model.State{{ID: "a", Values: emptyName}, {ID: "z", Values: emptyName}}}
	requireSameValidation(t, first, second, "empty_state_value_name", "a", "values")
}

func TestTransitionValidationOrderIsInvariant(t *testing.T) {
	states := []model.State{{ID: "z"}, {ID: "a"}}
	first := model.Transition{ID: "z-step", FromStateID: "missing-z", ToStateID: "a"}
	second := model.Transition{ID: "a-step", FromStateID: "missing-a", ToStateID: "a"}
	forward := model.Model{States: states, Transitions: []model.Transition{first, second}}
	reverse := model.Model{States: []model.State{states[1], states[0]}, Transitions: []model.Transition{second, first}}
	requireSameValidation(t, forward, reverse, "unknown_from_state", "a-step", "missing-a")
}

func TestValidationReferencesAreSorted(t *testing.T) {
	transition := model.Transition{ID: "z-step", FromStateID: "a-missing", ToStateID: "ready"}
	graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: []model.Transition{transition}}
	requireValidation(t, graph, "unknown_from_state", "a-missing", "z-step")
}

func TestIdentifierValidationOrderIsInvariant(t *testing.T) {
	first := model.Model{States: []model.State{{ID: "z bad"}, {ID: "a bad"}}}
	second := model.Model{States: []model.State{{ID: "a bad"}, {ID: "z bad"}}}
	requireSameValidation(t, first, second, "invalid_identifier", "a bad", "states.id")
}
