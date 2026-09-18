// Requirement validation binds every predicate identifier to the validated graph.
// It never treats an unknown identifier as a false predicate.
package proof

import "sort"

type requirementSets struct {
	badStates      map[string]struct{}
	badTransitions map[string]struct{}
	terminalStates map[string]struct{}
}

func validateRequirement(index graphIndex, requirement Requirement) (requirementSets, error) {
	if requirement.ID == "" {
		return requirementSets{}, proofError("empty_requirement_id", "requirement.id")
	}
	badStates, err := knownIDs(requirement.BadStateIDs, index.stateIDs, "bad_state")
	if err != nil {
		return requirementSets{}, err
	}
	badTransitions, err := knownIDs(requirement.BadTransitionIDs, index.transitionIDs, "bad_transition")
	if err != nil {
		return requirementSets{}, err
	}
	terminals, err := knownIDs(requirement.TerminalStateIDs, index.stateIDs, "terminal_state")
	if err != nil {
		return requirementSets{}, err
	}
	sets := requirementSets{badStates: badStates, badTransitions: badTransitions, terminalStates: terminals}
	return validateRequirementKind(requirement, sets)
}

func validateRequirementKind(requirement Requirement, sets requirementSets) (requirementSets, error) {
	switch requirement.Kind {
	case RequirementSafety:
		if len(sets.terminalStates) != 0 {
			return requirementSets{}, proofError("unexpected_terminal_state", requirement.ID)
		}
	case RequirementTotalTermination:
		if len(sets.badStates) != 0 || len(sets.badTransitions) != 0 {
			return requirementSets{}, proofError("unexpected_safety_predicate", requirement.ID)
		}
		if len(sets.terminalStates) == 0 {
			return requirementSets{}, proofError("empty_terminal_states", requirement.ID)
		}
	default:
		return requirementSets{}, proofError("unknown_requirement_kind", string(requirement.Kind))
	}
	return sets, nil
}

func knownIDs(values []string, known map[string]struct{}, field string) (map[string]struct{}, error) {
	ordered := append([]string(nil), values...)
	sort.Strings(ordered)
	result := make(map[string]struct{}, len(ordered))
	for _, id := range ordered {
		if id == "" {
			return nil, proofError("empty_identifier", field)
		}
		if _, exists := result[id]; exists {
			return nil, proofError("duplicate_identifier", field, id)
		}
		if _, exists := known[id]; !exists {
			return nil, proofError("unknown_identifier", field, id)
		}
		result[id] = struct{}{}
	}
	return result, nil
}
