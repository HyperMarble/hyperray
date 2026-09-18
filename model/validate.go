// This file validates the structural integrity of explicit graphs.
// It never computes reachability or rejects valid edge multiplicity.
package model

const (
	codeNoStates            = "no_states"
	codeEmptyStateID        = "empty_state_id"
	codeDuplicateStateID    = "duplicate_state_id"
	codeEmptyValueName      = "empty_state_value_name"
	codeEmptyTransitionID   = "empty_transition_id"
	codeDuplicateTransition = "duplicate_transition_id"
	codeInvalidIdentifier   = "invalid_identifier"
	codeUnknownFromState    = "unknown_from_state"
	codeUnknownToState      = "unknown_to_state"
)

// Validate returns the first structural error in canonical identifier order.
func Validate(graph Model) error {
	if len(graph.States) == 0 {
		return validationError(codeNoStates, "states")
	}
	ordered := orderedForValidation(graph)
	stateIDs := make(map[string]struct{}, len(ordered.States))
	for _, state := range ordered.States {
		if err := identifierError(state.ID, codeEmptyStateID, "states.id"); err != nil {
			return err
		}
		if _, exists := stateIDs[state.ID]; exists {
			return validationError(codeDuplicateStateID, state.ID)
		}
		if _, exists := state.Values[""]; exists {
			return validationError(codeEmptyValueName, state.ID, "values")
		}
		stateIDs[state.ID] = struct{}{}
	}
	transitionIDs := make(map[string]struct{}, len(ordered.Transitions))
	for _, transition := range ordered.Transitions {
		if err := identifierError(transition.ID, codeEmptyTransitionID, "transitions.id"); err != nil {
			return err
		}
		if _, exists := transitionIDs[transition.ID]; exists {
			return validationError(codeDuplicateTransition, transition.ID)
		}
		if err := stateReferenceError(transition.ID, transition.FromStateID,
			"transitions.from_state_id", codeUnknownFromState, stateIDs); err != nil {
			return err
		}
		if err := stateReferenceError(transition.ID, transition.ToStateID,
			"transitions.to_state_id", codeUnknownToState, stateIDs); err != nil {
			return err
		}
		transitionIDs[transition.ID] = struct{}{}
	}
	return nil
}
