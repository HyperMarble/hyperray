// Cycle search finds a deterministic repeatable path in the nonterminal graph.
// It never uses a terminal-state edge as nontermination evidence.
package proof

import "slices"

type cycleCounterexample struct {
	prefix searchPath
	cycle  searchPath
}

type cycleAction uint8

const (
	cycleIgnore cycleAction = iota
	cycleQueue
	cycleClose
)

func shortestNonterminalCycle(index graphIndex, reachable reachability,
	terminals map[string]struct{}) (cycleCounterexample, bool) {
	allowed := nonterminalStates(reachable, terminals)
	candidates := make([]cycleCounterexample, 0)
	for _, stateID := range reachable.stateIDs {
		if _, exists := allowed[stateID]; !exists {
			continue
		}
		prefix := reachable.paths[stateID]
		cycle, found := shortestCycleFrom(index, stateID, prefix.rootID, allowed)
		if found {
			candidates = append(candidates, cycleCounterexample{prefix: prefix, cycle: cycle})
		}
	}
	if len(candidates) == 0 {
		return cycleCounterexample{}, false
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if cycleLess(candidate, best) {
			best = candidate
		}
	}
	return best, true
}

func nonterminalStates(reachable reachability, terminals map[string]struct{}) map[string]struct{} {
	states := make(map[string]struct{}, len(reachable.stateIDs))
	for _, stateID := range reachable.stateIDs {
		if _, terminal := terminals[stateID]; !terminal {
			states[stateID] = struct{}{}
		}
	}
	return states
}

func cycleLess(left cycleCounterexample, right cycleCounterexample) bool {
	if len(left.prefix.transitionIDs) != len(right.prefix.transitionIDs) {
		return len(left.prefix.transitionIDs) < len(right.prefix.transitionIDs)
	}
	if len(left.cycle.transitionIDs) != len(right.cycle.transitionIDs) {
		return len(left.cycle.transitionIDs) < len(right.cycle.transitionIDs)
	}
	leftIDs := append(append([]string(nil), left.prefix.transitionIDs...), left.cycle.transitionIDs...)
	rightIDs := append(append([]string(nil), right.prefix.transitionIDs...), right.cycle.transitionIDs...)
	if order := slices.Compare(leftIDs, rightIDs); order != 0 {
		return order < 0
	}
	return pathLess(left.prefix, right.prefix)
}
