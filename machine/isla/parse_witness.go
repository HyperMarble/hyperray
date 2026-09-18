// Witness parsing reads counts and one concrete state from Herd output.
// It must reject a counterexample without a state.
package isla

import "strings"

func witnessCounts(lines []string) (uint64, uint64, error) {
	var positive, negative uint64
	found := false
	for index := range lines {
		line := lines[index]
		if !strings.HasPrefix(line, "Positive:") {
			continue
		}
		parsedPositive, parsedNegative, valid := canonicalCountLine(line)
		if !valid || found {
			return 0, 0, engineError(ProtocolError, "witnesses", line)
		}
		positive = parsedPositive
		negative = parsedNegative
		found = true
	}
	if !found || positive > ^uint64(0)-negative || positive+negative == 0 {
		return 0, 0, engineError(ProtocolError, "witnesses", "missing or invalid counts")
	}
	return positive, negative, nil
}

func resultState(lines []string, counterexamples uint64, otherCandidates uint64) (string, error) {
	if counterexamples == 0 {
		return "", nil
	}
	rows, err := candidateRows(lines, counterexamples, otherCandidates)
	if err != nil {
		return "", err
	}
	return identifiedCounterexample(rows, counterexamples, otherCandidates)
}

func consistentResult(status string, counterexamples uint64) error {
	allowed := status == "Allowed"
	found := counterexamples > 0
	if allowed == found {
		return nil
	}
	return engineError(ProtocolError, "result", "outcome and witness counts differ")
}
