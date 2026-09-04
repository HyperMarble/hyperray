// Proposal tests use two programs through the same public engine operation.
// They require different solver outcomes without production source changes.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestDifferentProgramFindsCounterexample(t *testing.T) {
	proposal, err := testEngine(t).Propose(t.Context(), testRequest(t, "counterexample"))
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if proposal.Status != isla.CounterexampleFound {
		t.Errorf("Status = %q", proposal.Status)
	}
	if proposal.QueryName != "different-code" || proposal.CounterexampleCount != 1 {
		t.Errorf("proposal = %#v", proposal)
	}
	if proposal.CounterexampleState != "x5=#x0000000000000003;" {
		t.Errorf("CounterexampleState = %q", proposal.CounterexampleState)
	}
}

func TestCorrectProgramHasNoCounterexample(t *testing.T) {
	proposal, err := testEngine(t).Propose(t.Context(), testRequest(t, "proof"))
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if proposal.Status != isla.NoCounterexampleFound {
		t.Errorf("Status = %q", proposal.Status)
	}
	if proposal.CandidateCount != 1 || proposal.CounterexampleState != "" {
		t.Errorf("proposal = %#v", proposal)
	}
	if proposal.Evidence.PCVisitLimit != 2 || proposal.Evidence.TimeLimitSeconds != 3 {
		t.Errorf("Evidence = %#v", proposal.Evidence)
	}
	digests := []string{
		proposal.Evidence.ArchitectureDigest,
		proposal.Evidence.ConfigurationDigest,
		proposal.Evidence.MemoryModelDigest,
		proposal.Evidence.ProgramDigest,
		proposal.Evidence.OutputDigest,
		proposal.Evidence.Tool.Digest,
	}
	for index := range digests {
		digest := digests[index]
		if len(digest) != 64 {
			t.Errorf("evidence digest = %q", digest)
		}
	}
}
