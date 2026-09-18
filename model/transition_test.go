// These tests exercise transition validation through the public package API.
// They never treat self-loops or parallel transitions as errors.
package model_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestRejectsEmptyTransitionID(t *testing.T) {
	graph := model.Model{
		States:      []model.State{{ID: "ready"}},
		Transitions: []model.Transition{{FromStateID: "ready", ToStateID: "ready"}},
	}
	requireValidation(t, graph, "empty_transition_id", "transitions.id")
}

func TestRejectsDuplicateTransitionID(t *testing.T) {
	edge := model.Transition{ID: "step", FromStateID: "ready", ToStateID: "ready"}
	graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: []model.Transition{edge, edge}}
	requireValidation(t, graph, "duplicate_transition_id", "step")
}

func TestRejectsUnknownFromState(t *testing.T) {
	edge := model.Transition{ID: "step", FromStateID: "missing", ToStateID: "ready"}
	graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: []model.Transition{edge}}
	requireValidation(t, graph, "unknown_from_state", "missing", "step")
}

func TestRejectsUnknownToState(t *testing.T) {
	edge := model.Transition{ID: "step", FromStateID: "ready", ToStateID: "missing"}
	graph := model.Model{States: []model.State{{ID: "ready"}}, Transitions: []model.Transition{edge}}
	requireValidation(t, graph, "unknown_to_state", "missing", "step")
}

func TestAcceptsSelfLoopsAndParallelTransitions(t *testing.T) {
	graph := model.Model{
		States: []model.State{{ID: "ready"}, {ID: "unreachable"}},
		Transitions: []model.Transition{
			{ID: "first", FromStateID: "ready", ToStateID: "ready"},
			{ID: "second", FromStateID: "ready", ToStateID: "ready"},
		},
	}
	if err := model.Validate(graph); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}
