// Witness parsing reads counts and one concrete state from Herd output.
// It must reject a counterexample without a state.
package isla

import (
	"strconv"
	"strings"
)

func witnessCounts(lines []string) (uint64, uint64, error) {
	for index := range lines {
		line := lines[index]
		fields := strings.Fields(line)
		if len(fields) != 4 || fields[0] != "Positive:" || fields[2] != "Negative:" {
			continue
		}
		positive, firstError := strconv.ParseUint(fields[1], 10, 64)
		negative, secondError := strconv.ParseUint(fields[3], 10, 64)
		if firstError != nil || secondError != nil {
			return 0, 0, engineError(ProtocolError, "witnesses", line)
		}
		return positive, negative, nil
	}
	return 0, 0, engineError(ProtocolError, "witnesses", "missing counts")
}

func resultState(lines []string, counterexamples uint64, otherCandidates uint64) (string, error) {
	if counterexamples == 0 {
		return "", nil
	}
	if otherCandidates != 0 {
		return "", engineError(ResultError, "counterexample", "Herd output does not identify the allowed state")
	}
	for index, line := range lines {
		if !strings.HasPrefix(line, "States ") || index+1 >= len(lines) {
			continue
		}
		state := strings.TrimSpace(lines[index+1])
		if state == "" || state == "???;" {
			return "", engineError(ProtocolError, "counterexample", "missing state")
		}
		return state, nil
	}
	return "", engineError(ProtocolError, "counterexample", "missing states")
}

func consistentResult(status string, counterexamples uint64) error {
	allowed := status == "Allowed"
	found := counterexamples > 0
	if allowed == found {
		return nil
	}
	return engineError(ProtocolError, "result", "outcome and witness counts differ")
}
