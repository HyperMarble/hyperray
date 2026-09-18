// Terminal evidence identifies one accepted candidate and modeled thread.
// It must never convert an exit, error, or discarded path into a boundary.
package isla

import "strconv"

// TerminalKind identifies the reason a native execution path terminated.
type TerminalKind string

const (
	// BoundaryReached means the declared continuation was reached.
	BoundaryReached TerminalKind = "boundary_reached"
)

// TerminalEvidence is the public native terminal record for one path.
type TerminalEvidence struct {
	CandidateIndex  uint64       `json:"candidate_index"`
	ThreadIndex     uint64       `json:"thread_index"`
	Kind            TerminalKind `json:"kind"`
	DeclaredAddress uint64       `json:"declared_address"`
}

func terminalRecordKey(record TerminalEvidence) string {
	return strconv.FormatUint(record.CandidateIndex, 10) + "/" + strconv.FormatUint(record.ThreadIndex, 10)
}

func parseTerminalEvidence(lines []string, candidateCount uint64) ([]TerminalEvidence, error) {
	var records []TerminalEvidence
	seen := make(map[string]struct{})
	for _, line := range lines {
		if !terminalLine(line) {
			continue
		}
		record, err := parseTerminalRecord(line)
		if err != nil {
			return nil, err
		}
		if record.CandidateIndex >= candidateCount {
			return nil, engineError(ProtocolError, "terminal evidence", "candidate index is outside States")
		}
		key := terminalRecordKey(record)
		if _, exists := seen[key]; exists {
			return nil, engineError(ProtocolError, "terminal evidence", "duplicate candidate/thread record")
		}
		seen[key] = struct{}{}
		records = append(records, record)
	}
	return records, nil
}
