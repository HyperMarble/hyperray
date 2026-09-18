// Reachability computes the exact least fixed point from validated roots.
// It never removes states because they are unrelated to one requirement.
package proof

import (
	"sort"

	"github.com/HyperMarble/hyperray/coverage"
)

type reachability struct {
	paths         map[string]searchPath
	stateIDs      []string
	transitionIDs []string
}

func explore(index graphIndex, roots []coverage.ValidatedRoot) reachability {
	pending := rootPaths(roots)
	paths := make(map[string]searchPath, len(index.states))
	transitions := make(map[string]struct{}, len(index.transitionIDs))
	for len(pending) != 0 {
		sortPaths(pending)
		path := pending[0]
		pending = pending[1:]
		stateID := lastStateID(path)
		if _, exists := paths[stateID]; exists {
			continue
		}
		paths[stateID] = path
		for _, transition := range index.outgoing[stateID] {
			transitions[transition.ID] = struct{}{}
			pending = append(pending, extendPath(path, transition))
		}
	}
	return reachability{
		paths:         paths,
		stateIDs:      sortedSetIDs(pathIDs(paths)),
		transitionIDs: sortedSetIDs(transitions),
	}
}

func rootPaths(roots []coverage.ValidatedRoot) []searchPath {
	paths := make([]searchPath, 0)
	for _, root := range roots {
		for _, stateID := range root.StateIDs {
			paths = append(paths, initialPath(root.ID, stateID))
		}
	}
	return paths
}

func pathIDs(paths map[string]searchPath) map[string]struct{} {
	ids := make(map[string]struct{}, len(paths))
	for stateID := range paths {
		ids[stateID] = struct{}{}
	}
	return ids
}

func sortedSetIDs(ids map[string]struct{}) []string {
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}
