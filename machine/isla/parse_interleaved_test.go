// Interleaved native rows carry one or more terminal records per candidate.
// The parser must preserve candidate order and reject grouped metadata.
package isla

import "testing"

func TestParseHerdResultReadsInterleavedThreadEvidence(t *testing.T) {
	output := "Test arm Allowed\nStates 3\nforbidden ???;\nTerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nTerminalEvidence v1 candidate_index=0 thread_index=1 kind=boundary_reached declared_address=0x0000000100008000\nallowed 0:R0=4;\nTerminalEvidence v1 candidate_index=1 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nforbidden ???;\nTerminalEvidence v1 candidate_index=2 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nTerminalEvidence v1 candidate_index=2 thread_index=1 kind=boundary_reached declared_address=0x0000000100008000\nPositive: 1 Negative: 2"
	result, err := parseHerdResult(output, "")
	if err != nil {
		t.Fatalf("parseHerdResult() error = %v", err)
	}
	if len(result.terminalEvidence) != 5 {
		t.Fatalf("terminal evidence count = %d, want 5", len(result.terminalEvidence))
	}
	if result.counterexampleState != "0:R0=4;" {
		t.Fatalf("counterexample state = %q, want concrete allowed row", result.counterexampleState)
	}
}

func TestParseHerdResultRejectsGroupedTerminalEvidence(t *testing.T) {
	output := "Test arm Allowed\nStates 2\nforbidden ???;\nallowed 0:R0=4;\nTerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nTerminalEvidence v1 candidate_index=1 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nPositive: 1 Negative: 1"
	if _, err := parseHerdResult(output, ""); err == nil {
		t.Fatal("accepted grouped terminal evidence")
	}
}

func TestParseHerdResultRejectsSwappedInterleavedCandidateIndexes(t *testing.T) {
	output := "Test arm Allowed\nStates 2\nforbidden ???;\nTerminalEvidence v1 candidate_index=1 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nallowed 0:R0=4;\nTerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nPositive: 1 Negative: 1"
	if _, err := parseHerdResult(output, ""); err == nil {
		t.Fatal("accepted terminal records with swapped candidate indexes")
	}
}
