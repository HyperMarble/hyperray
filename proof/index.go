// The graph index gives deterministic state and transition access.
// It never changes the validated graph.
package proof

import (
	"sort"

	"github.com/HyperMarble/hyperray/model"
)

type graphIndex struct {
	states        map[string]model.State
	outgoing      map[string][]model.Transition
	stateIDs      map[string]struct{}
	transitionIDs map[string]struct{}
}

func indexGraph(graph model.Model) graphIndex {
	index := graphIndex{
		states:        make(map[string]model.State, len(graph.States)),
		outgoing:      make(map[string][]model.Transition, len(graph.States)),
		stateIDs:      make(map[string]struct{}, len(graph.States)),
		transitionIDs: make(map[string]struct{}, len(graph.Transitions)),
	}
	for _, state := range graph.States {
		index.states[state.ID] = cloneState(state)
		index.stateIDs[state.ID] = struct{}{}
	}
	for _, transition := range graph.Transitions {
		index.outgoing[transition.FromStateID] = append(index.outgoing[transition.FromStateID], transition)
		index.transitionIDs[transition.ID] = struct{}{}
	}
	for stateID := range index.outgoing {
		sort.Slice(index.outgoing[stateID], func(left, right int) bool {
			return index.outgoing[stateID][left].ID < index.outgoing[stateID][right].ID
		})
	}
	return index
}

func cloneState(state model.State) model.State {
	values := make(map[string]string, len(state.Values))
	for name, value := range state.Values {
		values[name] = value
	}
	state.Values = values
	return state
}
