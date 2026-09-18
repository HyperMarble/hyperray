// This test measures canonical ordering without exposing it as public API.
// It never permits validation to mutate caller-owned slices.
package model

import "testing"

func TestOrderedForValidationIsStableAndNonmutating(t *testing.T) {
	graph := Model{
		States: []State{{ID: "z"}, {ID: "a"}},
		Transitions: []Transition{
			{ID: "z", FromStateID: "z", ToStateID: "a"},
			{ID: "a", FromStateID: "a", ToStateID: "z"},
		},
	}
	ordered := orderedForValidation(graph)
	if ordered.States[0].ID != "a" || ordered.Transitions[0].ID != "a" {
		t.Errorf("ordered graph starts with state %q and transition %q",
			ordered.States[0].ID, ordered.Transitions[0].ID)
	}
	if graph.States[0].ID != "z" || graph.Transitions[0].ID != "z" {
		t.Error("orderedForValidation() mutated caller slices")
	}
}

func TestStateBeforeUsesIDThenEmptyName(t *testing.T) {
	if !stateBefore(State{ID: "a"}, State{ID: "z"}) {
		t.Error("stateBefore() did not order identifiers")
	}
	withEmptyName := State{ID: "same", Values: map[string]string{"": "value"}}
	if !stateBefore(withEmptyName, State{ID: "same"}) {
		t.Error("stateBefore() did not order the validation-relevant value name")
	}
}

func TestTransitionBeforeUsesEveryIdentifier(t *testing.T) {
	base := Transition{ID: "same", FromStateID: "same", ToStateID: "same"}
	if !transitionBefore(Transition{ID: "a"}, Transition{ID: "z"}) {
		t.Error("transitionBefore() did not order transition identifiers")
	}
	if !transitionBefore(Transition{ID: "same", FromStateID: "a"}, base) {
		t.Error("transitionBefore() did not order source identifiers")
	}
	if !transitionBefore(Transition{ID: "same", FromStateID: "same", ToStateID: "a"}, base) {
		t.Error("transitionBefore() did not order target identifiers")
	}
}
