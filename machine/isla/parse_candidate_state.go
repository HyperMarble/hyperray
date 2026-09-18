// One candidate row carries an explicit result and a concrete allowed state.
// Legacy rows are permitted only when no forbidden candidate exists.
package isla

import "strings"

func candidateState(row string, allowLegacy bool) (string, string, error) {
	status, state, labeled := strings.Cut(strings.TrimSpace(row), " ")
	if !labeled && allowLegacy {
		status = "allowed"
		state = strings.TrimSpace(row)
	}
	switch status {
	case "allowed":
		if !concreteState(state) {
			return "", "", engineError(ProtocolError, "counterexample", "allowed row has no concrete state")
		}
	case "forbidden":
		if state != "???;" {
			return "", "", engineError(ProtocolError, "counterexample", "forbidden row contains a state")
		}
	default:
		return "", "", engineError(ResultError, "counterexample", "candidate status is not identified")
	}
	return status, state, nil
}
