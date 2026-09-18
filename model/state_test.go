// These tests exercise state validation through the public package API.
// They never depend on unexported model details.
package model_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestRejectsNoStates(t *testing.T) {
	requireValidation(t, model.Model{}, "no_states", "states")
}

func TestRejectsEmptyStateID(t *testing.T) {
	graph := model.Model{States: []model.State{{}}}
	requireValidation(t, graph, "empty_state_id", "states.id")
}

func TestRejectsDuplicateStateID(t *testing.T) {
	graph := model.Model{States: []model.State{{ID: "same"}, {ID: "same"}}}
	requireValidation(t, graph, "duplicate_state_id", "same")
}

func TestRejectsEmptyStateValueName(t *testing.T) {
	state := model.State{ID: "ready", Values: map[string]string{"": "lost"}}
	requireValidation(t, model.Model{States: []model.State{state}},
		"empty_state_value_name", "ready", "values")
}
