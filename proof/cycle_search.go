// Local cycle search returns the shortest nonempty path back to one state.
// It never crosses a state outside the allowed nonterminal set.
package proof

func shortestCycleFrom(index graphIndex, stateID string, rootID string,
	allowed map[string]struct{}) (searchPath, bool) {
	pending := []searchPath{initialPath(rootID, stateID)}
	visited := make(map[string]struct{}, len(allowed))
	closed := make([]searchPath, 0)
	for len(pending) != 0 {
		sortPaths(pending)
		path := pending[0]
		pending = pending[1:]
		currentID := lastStateID(path)
		if _, exists := visited[currentID]; exists {
			continue
		}
		visited[currentID] = struct{}{}
		for _, transition := range index.outgoing[currentID] {
			next := extendPath(path, transition)
			switch classifyCycleStep(transition.ToStateID, stateID, allowed, visited) {
			case cycleQueue:
				pending = append(pending, next)
			case cycleClose:
				closed = append(closed, next)
			case cycleIgnore:
			}
		}
	}
	if len(closed) == 0 {
		return searchPath{}, false
	}
	sortPaths(closed)
	return closed[0], true
}

func classifyCycleStep(targetID string, entryID string, allowed map[string]struct{},
	visited map[string]struct{}) cycleAction {
	if _, exists := allowed[targetID]; !exists {
		return cycleIgnore
	}
	if targetID == entryID {
		return cycleClose
	}
	if _, exists := visited[targetID]; exists {
		return cycleIgnore
	}
	return cycleQueue
}
