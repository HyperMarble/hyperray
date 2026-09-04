// Herd-result parsing accepts the documented Isla result records.
// It must reject unknown outcomes and inconsistent candidate counts.
package isla

import "strings"

type herdResult struct {
	name                string
	candidates          uint64
	counterexamples     uint64
	counterexampleState string
}

func parseHerdResult(output string, diagnostics string) (herdResult, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	name, status, err := resultHeader(lines, diagnostics)
	if err != nil {
		return herdResult{}, err
	}
	positive, negative, err := witnessCounts(lines)
	if err != nil {
		return herdResult{}, err
	}
	state, err := resultState(lines, positive, negative)
	if err != nil {
		return herdResult{}, err
	}
	if err := consistentResult(status, positive); err != nil {
		return herdResult{}, err
	}
	return herdResult{name: name, candidates: positive + negative, counterexamples: positive, counterexampleState: state}, nil
}

func resultHeader(lines []string, diagnostics string) (string, string, error) {
	if len(lines) == 0 {
		return "", "", engineError(ProtocolError, "result", "empty output")
	}
	fields := strings.Fields(lines[0])
	if len(fields) != 3 || fields[0] != "Test" {
		return "", "", engineError(ProtocolError, "result", lines[0])
	}
	if fields[2] == "Error" {
		detail := strings.TrimSpace(diagnostics)
		if detail == "" {
			detail = "Isla reported an error"
		}
		return "", "", engineError(ResultError, fields[1], detail)
	}
	if fields[2] != "Allowed" && fields[2] != "Forbidden" {
		return "", "", engineError(ProtocolError, fields[1], fields[2])
	}
	return fields[1], fields[2], nil
}
