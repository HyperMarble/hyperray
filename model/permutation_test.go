// These tests cover every ordering of three validation-relevant graph items.
// They never accept a first error selected by caller input order.
package model_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestStateValidationOrderAcrossAllPermutations(t *testing.T) {
	first := model.State{ID: "a", Values: map[string]string{"": "first"}}
	second := model.State{ID: "m", Values: map[string]string{"": "second"}}
	third := model.State{ID: "z", Values: map[string]string{"": "third"}}
	permutations := [][]model.State{
		{first, second, third},
		{first, third, second},
		{second, first, third},
		{second, third, first},
		{third, first, second},
		{third, second, first},
	}
	for _, states := range permutations {
		graph := model.Model{States: states}
		requireValidation(t, graph, "empty_state_value_name", "a", "values")
	}
}

func TestTransitionValidationOrderAcrossAllPermutations(t *testing.T) {
	first := model.Transition{ID: "same", FromStateID: "a-missing", ToStateID: "ready"}
	second := model.Transition{ID: "same", FromStateID: "ready", ToStateID: "ready"}
	third := model.Transition{ID: "same", FromStateID: "z-missing", ToStateID: "ready"}
	permutations := [][]model.Transition{
		{first, second, third},
		{first, third, second},
		{second, first, third},
		{second, third, first},
		{third, first, second},
		{third, second, first},
	}
	for _, transitions := range permutations {
		graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: transitions}
		requireValidation(t, graph, "unknown_from_state", "a-missing", "same")
	}
}
