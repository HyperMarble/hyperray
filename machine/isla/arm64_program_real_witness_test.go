//go:build isla_integration && arm64_acceptance

// This helper checks real ARM64 terminal rows and solver witness assignments.
// It must not accept substring matches or invented register values.
package isla_test

import (
	"fmt"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func assertARM64TerminalEvidence(t *testing.T, result isla.ExecutableResult, address uint64) {
	t.Helper()
	rows := result.Verification.TerminalEvidence
	candidates := result.Verification.CandidateCount
	threads := result.Program.ThreadCount
	t.Logf("terminal evidence: candidates=%d rows=%#v", candidates, rows)
	if candidates == 0 || threads != 1 || uint64(len(rows)) != candidates {
		t.Fatalf("terminal evidence rows = %d, candidates = %d, threads = %d", len(rows), candidates, threads)
	}
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.CandidateIndex >= candidates || row.ThreadIndex >= threads || row.Kind != isla.BoundaryReached || row.DeclaredAddress != address {
			t.Fatalf("terminal evidence row = %#v", row)
		}
		key := fmt.Sprintf("%d/%d", row.CandidateIndex, row.ThreadIndex)
		if seen[key] {
			t.Fatalf("duplicate terminal evidence row %s", key)
		}
		seen[key] = true
	}
}

func assertARM64StatusCounts(t *testing.T, result isla.ExecutableResult, status isla.VerificationStatus) {
	t.Helper()
	verification := result.Verification
	if verification.Status != status {
		t.Fatalf("verification status = %q, want %q", verification.Status, status)
	}
	switch status {
	case isla.Proved:
		if verification.CounterexampleCount != 0 {
			t.Fatalf("proved counterexample count = %d, want 0", verification.CounterexampleCount)
		}
	case isla.Disproved:
		if verification.CounterexampleCount == 0 || verification.CounterexampleCount > verification.CandidateCount {
			t.Fatalf("disproved counterexample count = %d, candidates = %d", verification.CounterexampleCount, verification.CandidateCount)
		}
	default:
		t.Fatalf("unsupported verification status %q", status)
	}
}

func assertARM64Witness(t *testing.T, state string, returnAddress uint64) {
	t.Helper()
	t.Logf("counterexample witness: %s", state)
	assignments, err := arm64Assignments(state)
	if err != nil {
		t.Fatalf("counterexample assignments: %v", err)
	}
	if value, err := arm64WitnessValue(assignments, arm64R0Names(t)); err != nil || value != 4 {
		t.Fatalf("counterexample R0 is not 4: %s", state)
	}
	if value, err := arm64WitnessValue(assignments, []string{"0:SP_EL0"}); err != nil || value != 0x3c40 {
		t.Fatalf("counterexample SP_EL0 is not 0x3c40: %s", state)
	}
	if value, err := arm64WitnessValue(assignments, []string{"0:_PC"}); err != nil || value != returnAddress {
		t.Fatalf("counterexample _PC is not %#x: %s", returnAddress, state)
	}
}
