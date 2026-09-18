// Call observations must belong to the solver-selected counterexample.
// Missing or malformed observations cannot satisfy a requested safety contract.
package isla

import "strings"

func counterexampleCalls(names []string, proposal Proposal) (map[string]bool, error) {
	if proposal.Status != CounterexampleFound {
		return nil, nil
	}
	expected := make(map[string]bool, len(names))
	for _, name := range names {
		expected[name] = true
	}
	observations := make(map[string]bool, len(names))
	for _, assignment := range strings.Split(proposal.CounterexampleState, ";") {
		if !strings.HasPrefix(assignment, "called:") {
			continue
		}
		name, value, found := strings.Cut(strings.TrimPrefix(assignment, "called:"), "=")
		_, duplicate := observations[name]
		if !found || !expected[name] || duplicate || value != "true" && value != "false" {
			return nil, engineError(ProtocolError, "model calls", "invalid observation: "+assignment)
		}
		observations[name] = value == "true"
	}
	if len(observations) != len(expected) {
		return nil, engineError(ProtocolError, "model calls", "counterexample lacks a required observation")
	}
	return observations, nil
}
