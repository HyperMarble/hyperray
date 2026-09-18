// Terminal records use one strict versioned stdout grammar.
// Unknown versions and kinds must remain protocol errors.
package isla

import (
	"strconv"
	"strings"
)

func parseTerminalRecord(line string) (TerminalEvidence, error) {
	if line != strings.TrimSpace(line) || strings.ContainsAny(line, "\t\r") {
		return TerminalEvidence{}, engineError(ProtocolError, "terminal evidence", "invalid record whitespace")
	}
	fields := strings.Split(line, " ")
	if len(fields) != 6 || fields[0] != "TerminalEvidence" || fields[1] != "v1" {
		return TerminalEvidence{}, engineError(ProtocolError, "terminal evidence", "invalid versioned record")
	}
	candidate, err := decimalField(fields[2], "candidate_index")
	if err != nil {
		return TerminalEvidence{}, err
	}
	thread, err := decimalField(fields[3], "thread_index")
	if err != nil {
		return TerminalEvidence{}, err
	}
	if fields[4] != "kind=boundary_reached" {
		return TerminalEvidence{}, engineError(ProtocolError, "terminal evidence", "unsupported terminal kind")
	}
	address, err := terminalAddress(fields[5])
	if err != nil {
		return TerminalEvidence{}, err
	}
	return TerminalEvidence{CandidateIndex: candidate, ThreadIndex: thread, Kind: BoundaryReached, DeclaredAddress: address}, nil
}

func decimalField(field string, name string) (uint64, error) {
	key, value, found := strings.Cut(field, "=")
	if !found || key != name || value == "" {
		return 0, engineError(ProtocolError, "terminal evidence", "invalid "+name)
	}
	parsed, valid := canonicalDecimal(value)
	if !valid {
		return 0, engineError(ProtocolError, "terminal evidence", "invalid "+name)
	}
	return parsed, nil
}

func terminalAddress(field string) (uint64, error) {
	if !strings.HasPrefix(field, "declared_address=0x") {
		return 0, engineError(ProtocolError, "terminal evidence", "invalid declared address")
	}
	digits := strings.TrimPrefix(field, "declared_address=0x")
	if len(digits) != 16 {
		return 0, engineError(ProtocolError, "terminal evidence", "declared address is not 16 hex digits")
	}
	for index := range digits {
		character := digits[index]
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return 0, engineError(ProtocolError, "terminal evidence", "declared address is not lowercase hex")
		}
	}
	parsed, err := strconv.ParseUint(digits, 16, 64)
	if err != nil {
		return 0, engineError(ProtocolError, "terminal evidence", "invalid declared address")
	}
	return parsed, nil
}
