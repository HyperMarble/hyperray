// Model ordering tests make duplicate-state and transition validation stable.
// They never mutate the caller's graph while ordering it.
package coverage_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestDuplicateStateOrderInvariant(t *testing.T) {
	first := completeRequest(t)
	first.Model.States = []model.State{{ID: "same"}, {ID: "same", Values: map[string]string{"": "bad"}}}
	second := first
	second.Model.States = []model.State{first.Model.States[1], first.Model.States[0]}
	firstError := requireCoverageError(t, first, "invalid_model")
	secondError := requireCoverageError(t, second, "invalid_model")
	if !reflect.DeepEqual(firstError.References, secondError.References) {
		t.Errorf("state errors = %v, want %v", secondError.References, firstError.References)
	}
}

func TestDuplicateTransitionOrdering(t *testing.T) {
	request := completeRequest(t)
	request.Model.Transitions = []model.Transition{
		{ID: "same", FromStateID: "work", ToStateID: "start"},
		{ID: "same", FromStateID: "start", ToStateID: "work"},
	}
	requireCoverageError(t, request, "invalid_model")
	request = completeRequest(t)
	request.Model.Transitions = []model.Transition{
		{ID: "same", FromStateID: "start", ToStateID: "work"},
		{ID: "same", FromStateID: "start", ToStateID: "start"},
	}
	requireCoverageError(t, request, "invalid_model")
}
