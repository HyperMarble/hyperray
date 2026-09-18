// Safety proof selects the shortest deterministic reachable violation.
// It never reports an unreachable predicate match.
package proof

import (
	"slices"

	"github.com/HyperMarble/hyperray/coverage"
)

type safetyViolation struct {
	kind         WitnessKind
	path         searchPath
	stateID      string
	transitionID string
}

func proveSafety(index graphIndex, reachable reachability, report coverage.Report,
	requirement Requirement, sets requirementSets) Result {
	violations := safetyViolations(index, reachable, sets)
	result := baseResult(VerdictProved, report, requirement, reachable)
	if len(violations) == 0 {
		return result
	}
	best := violations[0]
	for _, violation := range violations[1:] {
		if safetyViolationLess(violation, best) {
			best = violation
		}
	}
	result.Verdict = VerdictDisproved
	result.Witness = safetyWitness(index, requirement.ID, best)
	return result
}

func safetyViolations(index graphIndex, reachable reachability, sets requirementSets) []safetyViolation {
	violations := make([]safetyViolation, 0)
	for _, stateID := range reachable.stateIDs {
		path := reachable.paths[stateID]
		if _, exists := sets.badStates[stateID]; exists {
			violations = append(violations, safetyViolation{kind: WitnessSafetyState, path: path, stateID: stateID})
		}
		for _, transition := range index.outgoing[stateID] {
			if _, exists := sets.badTransitions[transition.ID]; exists {
				violations = append(violations, safetyViolation{
					kind: WitnessSafetyTransition, path: extendPath(path, transition),
					stateID: transition.ToStateID, transitionID: transition.ID,
				})
			}
		}
	}
	return violations
}

func safetyViolationLess(left safetyViolation, right safetyViolation) bool {
	if pathLess(left.path, right.path) {
		return true
	}
	if pathLess(right.path, left.path) {
		return false
	}
	leftIDs := []string{string(left.kind), left.stateID, left.transitionID}
	rightIDs := []string{string(right.kind), right.stateID, right.transitionID}
	return slices.Compare(leftIDs, rightIDs) < 0
}
