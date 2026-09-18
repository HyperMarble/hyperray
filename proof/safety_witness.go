// Safety witnesses contain enough state and transition data for exact replay.
// They never replace a predicate cause with generic solver text.
package proof

func safetyWitness(index graphIndex, requirementID string, violation safetyViolation) *Witness {
	cause := "reachable state violates the safety requirement"
	if violation.kind == WitnessSafetyTransition {
		cause = "reachable transition violates the safety requirement"
	}
	return &Witness{
		Kind: violation.kind, RootID: violation.path.rootID,
		RequirementID: requirementID, Cause: cause,
		StateID: violation.stateID, TransitionID: violation.transitionID,
		Path: traceFor(index, violation.path),
	}
}
