package enforce

import (
	"github.com/HyperMarble/ray/internal/mutate"
)

// FalseNegatives asks the mirror of Discover's question. Discover hunts wrong
// solutions the tests accept; this hunts allowed variants the tests reject.
//
// A mutant that deviates on NO probe is, as far as the spec's observations
// reach, the same solution: every behaviour the spec watches is unchanged.
// The spec is the contract, so such a variant is permitted. If the task's
// verifier rejects it anyway, the tests are stricter than the contract --
// or the spec observes too little. Either way the disagreement is real and
// the finding names both suspects: the killing tests and the unobserved edit.
//
// The claim is deliberately bounded: "indistinguishable" means on the probes
// that exist, never in any absolute sense, and each finding says so.
func FalseNegatives(task Task, solutionFile string, mutants []mutate.Mutant, probes []Probe) ([]Deviation, error) {
	if len(probes) == 0 {
		return nil, nil
	}
	path := task.SourceRoot + "/" + solutionFile
	original, err := readSource(task, path)
	if err != nil {
		return nil, err
	}
	baseline := observe(task, probes)

	var found []Deviation
	for _, m := range mutants {
		if m.Source == "" || m.Source == original {
			continue
		}
		if err := writeSource(task, path, m.Source); err != nil {
			continue
		}
		after := observe(task, probes)
		_, deviates := firstDifference(probes, baseline, after)
		if deviates {
			// Observably different: Discover's territory, not this check's.
			_ = writeSource(task, path, original)
			continue
		}
		passed, out := verifierPasses(task, nil, solutionFile, m.Line)
		_ = writeSource(task, path, original)
		if !passed {
			found = append(found, Deviation{
				Mutant:  m,
				Checked: true,
				Reason:  "no probe distinguishes this variant from the reference, yet the verifier rejects it",
				Killers: FailedTestNames(out),
			})
		}
	}
	if err := writeSource(task, path, original); err != nil {
		return found, err
	}
	return found, nil
}
