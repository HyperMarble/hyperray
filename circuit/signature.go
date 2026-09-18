// Signature comparison makes both sides use the same step boundary.
// It never hides a missing variable or output inside the SMT miter.
package circuit

func signatureError(reference StepRelation, candidate StepRelation) error {
	if err := relationError(reference); err != nil {
		return err
	}
	if err := relationError(candidate); err != nil {
		return err
	}
	if !sameVariables(reference.states, candidate.states) {
		return engineError("state_signature_mismatch", "relations")
	}
	if !sameVariables(reference.inputs, candidate.inputs) {
		return engineError("input_signature_mismatch", "relations")
	}
	if !sameObservations(reference.observations, candidate.observations) {
		return engineError("observation_signature_mismatch", "relations")
	}
	return nil
}

func sameVariables(left []Variable, right []Variable) bool {
	if len(left) != len(right) {
		return false
	}
	for position := range left {
		if left[position] != right[position] {
			return false
		}
	}
	return true
}

func sameObservations(left []Observation, right []Observation) bool {
	if len(left) != len(right) {
		return false
	}
	for position := range left {
		if left[position].name != right[position].name {
			return false
		}
	}
	return true
}
