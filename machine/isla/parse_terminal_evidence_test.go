// Terminal records are versioned, exact, and tied to candidate rows.
// Malformed or duplicate records must not become accepted evidence.
package isla

import "testing"

func TestParseHerdResultReadsTerminalEvidence(t *testing.T) {
	output := "Test arm Allowed\nStates 2\nforbidden ???;\nTerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nallowed 0:R0=4;\nTerminalEvidence v1 candidate_index=1 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nPositive: 1 Negative: 1"
	result, err := parseHerdResult(output, "")
	if err != nil {
		t.Fatalf("parseHerdResult() error = %v", err)
	}
	if len(result.terminalEvidence) != 2 {
		t.Fatalf("terminal evidence count = %d, want 2", len(result.terminalEvidence))
	}
	want := TerminalEvidence{CandidateIndex: 1, ThreadIndex: 0, Kind: BoundaryReached, DeclaredAddress: 0x100008000}
	if result.terminalEvidence[1] != want {
		t.Errorf("terminal evidence = %#v, want %#v", result.terminalEvidence[1], want)
	}
}

func TestParseHerdResultReadsInterleavedTerminalEvidence(t *testing.T) {
	output := "Test arm Allowed\nStates 2\nforbidden ???;\nTerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nallowed 0:R0=4;\nTerminalEvidence v1 candidate_index=1 thread_index=0 kind=boundary_reached declared_address=0x0000000100008000\nPositive: 1 Negative: 1"
	result, err := parseHerdResult(output, "")
	if err != nil {
		t.Fatalf("parseHerdResult() error = %v", err)
	}
	if result.counterexamples != 1 || result.candidates != 2 {
		t.Fatalf("result counts = %#v, want one counterexample and two candidates", result)
	}
	if result.counterexampleState != "0:R0=4;" {
		t.Fatalf("counterexample state = %q, want concrete allowed row", result.counterexampleState)
	}
}

func TestParseHerdResultRejectsMalformedTerminalEvidence(t *testing.T) {
	cases := []string{
		"TerminalEvidence v2 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001",
		"TerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x1",
		"TerminalEvidence v1 candidate_index=+0 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001",
		"TerminalEvidence v1 candidate_index=00 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001",
		"TerminalEvidence v1 candidate_index=0 thread_index=00 kind=boundary_reached declared_address=0x0000000000000001",
		"TerminalEvidence v1 candidate_index=18446744073709551616 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001",
		"TerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x000000000000000A",
		"TerminalEvidence\tv1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001",
		"TerminalEvidenceX v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001",
		"prefix TerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001",
		"TerminalEvidence v1 candidate_index=0 thread_index=0 kind=unknown declared_address=0x0000000000000001",
		"TerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001 extra=x",
		"TerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001\nTerminalEvidence v1 candidate_index=0 thread_index=0 kind=boundary_reached declared_address=0x0000000000000001",
	}
	for index, record := range cases {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			output := "Test arm Forbidden\nStates 1\nforbidden ???;\n" + record + "\nPositive: 0 Negative: 1"
			if _, err := parseHerdResult(output, ""); err == nil {
				t.Fatalf("accepted malformed terminal record %q", record)
			}
		})
	}
}

func TestParseHerdResultKeepsRVTerminalEvidenceAbsent(t *testing.T) {
	output := "Test rv Forbidden\nStates 1\nforbidden ???;\nPositive: 0 Negative: 1"
	result, err := parseHerdResult(output, "")
	if err != nil {
		t.Fatalf("parseHerdResult() error = %v", err)
	}
	if result.terminalEvidence != nil {
		t.Fatalf("terminal evidence = %#v, want nil", result.terminalEvidence)
	}
}

func TestParseHerdResultDoesNotTreatHeaderAsTerminalEvidence(t *testing.T) {
	output := "Test TerminalEvidenceCase Forbidden\nStates 1\nforbidden ???;\nPositive: 0 Negative: 1"
	if _, err := parseHerdResult(output, ""); err != nil {
		t.Fatalf("parseHerdResult() rejected header containing terminal text: %v", err)
	}
}
