// Termination proof rejects reachable deadlocks and nonterminal cycles.
// It treats entry into a terminal state as successful completion.
package proof

import "github.com/HyperMarble/hyperray/coverage"

func proveTermination(index graphIndex, reachable reachability, report coverage.Report,
	requirement Requirement, sets requirementSets) Result {
	result := baseResult(VerdictProved, report, requirement, reachable)
	deadlock, found := shortestDeadlock(index, reachable, sets.terminalStates)
	if found {
		result.Verdict = VerdictDisproved
		result.Witness = deadlockWitness(index, requirement.ID, deadlock)
		return result
	}
	cycle, found := shortestNonterminalCycle(index, reachable, sets.terminalStates)
	if found {
		result.Verdict = VerdictDisproved
		result.Witness = cycleWitness(index, requirement.ID, cycle)
	}
	return result
}

func shortestDeadlock(index graphIndex, reachable reachability,
	terminals map[string]struct{}) (searchPath, bool) {
	paths := make([]searchPath, 0)
	for _, stateID := range reachable.stateIDs {
		if _, terminal := terminals[stateID]; terminal {
			continue
		}
		if len(index.outgoing[stateID]) == 0 {
			paths = append(paths, reachable.paths[stateID])
		}
	}
	if len(paths) == 0 {
		return searchPath{}, false
	}
	sortPaths(paths)
	return paths[0], true
}

func deadlockWitness(index graphIndex, requirementID string, path searchPath) *Witness {
	return &Witness{
		Kind: WitnessTerminationDeadlock, RootID: path.rootID,
		RequirementID: requirementID,
		Cause:         "reachable nonterminal state has no outgoing transition",
		StateID:       lastStateID(path), Path: traceFor(index, path),
	}
}
