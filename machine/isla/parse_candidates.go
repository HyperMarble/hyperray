// Candidate rows bind each labeled state to a solver outcome.
// Counts, labels, and concrete values must agree before a witness is returned.
package isla

import "strings"

func candidateRows(lines []string, positive uint64, negative uint64) ([]string, error) {
	if positive > ^uint64(0)-negative {
		return nil, engineError(ProtocolError, "states", "candidate count overflow")
	}
	start := -1
	var count uint64
	for index, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "States ") {
			continue
		}
		parsed, valid := canonicalStateCount(line)
		if !valid || start >= 0 {
			return nil, engineError(ProtocolError, "states", "invalid or duplicate header")
		}
		start = index + 1
		count = parsed
	}
	if start < 0 || count != positive+negative || count == 0 {
		return nil, engineError(ProtocolError, "states", "missing or inconsistent candidate rows")
	}
	if terminalBeforeState(lines, start) {
		return nil, engineError(ProtocolError, "states", "terminal evidence is before candidate rows")
	}
	rows, end, terminalRows, err := interleavedCandidateRows(lines[start:], count)
	if err != nil {
		return nil, err
	}
	if terminalRows != 0 && terminalRows != count {
		return nil, engineError(ProtocolError, "states", "terminal evidence is not interleaved with every row")
	}
	if terminalAfterCandidates(lines[start+end:]) {
		return nil, engineError(ProtocolError, "states", "terminal evidence is outside candidate rows")
	}
	return rows, nil
}

func terminalBeforeState(lines []string, stateStart int) bool {
	for _, line := range lines[:stateStart-1] {
		if terminalLine(line) {
			return true
		}
	}
	return false
}

func interleavedCandidateRows(lines []string, count uint64) ([]string, int, uint64, error) {
	rows := make([]string, 0)
	index := 0
	terminalRows := uint64(0)
	for uint64(len(rows)) < count {
		if index >= len(lines) || terminalLine(lines[index]) {
			return nil, 0, 0, engineError(ProtocolError, "states", "missing candidate row")
		}
		rows = append(rows, lines[index])
		index++
		foundTerminal := false
		for index < len(lines) && terminalLine(lines[index]) {
			record, err := parseTerminalRecord(lines[index])
			if err != nil {
				return nil, 0, 0, err
			}
			if record.CandidateIndex != uint64(len(rows)-1) {
				return nil, 0, 0, engineError(ProtocolError, "states", "terminal evidence candidate index differs from row")
			}
			foundTerminal = true
			index++
		}
		if foundTerminal {
			terminalRows++
			if terminalRows != uint64(len(rows)) {
				return nil, 0, 0, engineError(ProtocolError, "states", "terminal evidence is not interleaved with every row")
			}
		}
	}
	return rows, index, terminalRows, nil
}

func terminalAfterCandidates(lines []string) bool {
	for _, line := range lines {
		if terminalLine(line) {
			return true
		}
	}
	return false
}

func identifiedCounterexample(rows []string, positive uint64, negative uint64) (string, error) {
	var allowed, forbidden uint64
	witness := ""
	for _, row := range rows {
		status, state, err := candidateState(row, negative == 0)
		if err != nil {
			return "", err
		}
		switch status {
		case "allowed":
			allowed++
			witness = state
		case "forbidden":
			forbidden++
		default:
			return "", engineError(ResultError, "counterexample", "candidate status is not identified")
		}
	}
	if allowed != positive || forbidden != negative {
		return "", engineError(ProtocolError, "counterexample", "candidate status counts differ")
	}
	return witness, nil
}
