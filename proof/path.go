// Search paths retain one exact root-to-state execution.
// They never omit a transition from a witness.
package proof

import (
	"slices"
	"sort"

	"github.com/HyperMarble/hyperray/model"
)

type searchPath struct {
	rootID        string
	stateIDs      []string
	transitionIDs []string
}

func initialPath(rootID string, stateID string) searchPath {
	return searchPath{rootID: rootID, stateIDs: []string{stateID}}
}

func extendPath(path searchPath, transition model.Transition) searchPath {
	states := append([]string(nil), path.stateIDs...)
	transitions := append([]string(nil), path.transitionIDs...)
	states = append(states, transition.ToStateID)
	transitions = append(transitions, transition.ID)
	return searchPath{rootID: path.rootID, stateIDs: states, transitionIDs: transitions}
}

func pathLess(left searchPath, right searchPath) bool {
	if len(left.transitionIDs) != len(right.transitionIDs) {
		return len(left.transitionIDs) < len(right.transitionIDs)
	}
	if order := slices.Compare(left.transitionIDs, right.transitionIDs); order != 0 {
		return order < 0
	}
	if left.rootID != right.rootID {
		return left.rootID < right.rootID
	}
	return slices.Compare(left.stateIDs, right.stateIDs) < 0
}

func sortPaths(paths []searchPath) {
	sort.Slice(paths, func(left, right int) bool {
		return pathLess(paths[left], paths[right])
	})
}

func lastStateID(path searchPath) string {
	return path.stateIDs[len(path.stateIDs)-1]
}

func traceFor(index graphIndex, path searchPath) Trace {
	states := make([]model.State, 0, len(path.stateIDs))
	for _, stateID := range path.stateIDs {
		states = append(states, cloneState(index.states[stateID]))
	}
	transitions := append([]string(nil), path.transitionIDs...)
	return Trace{States: states, TransitionIDs: transitions}
}
