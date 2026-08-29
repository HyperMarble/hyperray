package tests

import (
	"testing"

	"github.com/HyperMarble/hyperray/internal/depharvest"
	"github.com/HyperMarble/hyperray/internal/difftest"
)

// TestPipeline_VerifyFixReverifyLoop tests hyperray's actual purpose: given a
// built task, either hand back real issues, or say the task is ready.
//
// Both halves have to be trustworthy or the loop is worthless. If the
// issues are artifacts, the report is noise. If hyperray goes quiet on code
// that is still broken, "ready" is a lie -- and that is the more
// dangerous failure, because it is silent.
//
// So this runs three rounds, and the third is the load-bearing one:
//
//  1. the agent's submission, carrying a bug that survives review
//  2. the same code after the fix hyperray's report pointed at
//  3. a CONTROL -- a different bug injected into the fixed code
//
// Round 2 going quiet only means something if round 3 does not. Without
// the control, silence and blindness are indistinguishable.
//
// Measured: round 1 finds the IndexError on empty/blank strings, round 2
// finds nothing, round 3 finds 150 disagreements. The silence is real.
func TestPipeline_VerifyFixReverifyLoop(t *testing.T) {
	python := testPython3(t)

	// The proven reference model -- what Layer 3 establishes is correct.
	const model = `
def normalize(s):
    s = s.strip()
    if not s:
        return ""
    return s[0].upper() + s[1:]
`

	h, err := depharvest.Run(testHarvestPython(t), "jsonschema", testHarvestTestsPath(t), 60000)
	if err != nil {
		t.Fatalf("depharvest: %v", err)
	}
	var inputs [][]any
	for _, v := range h.Values["str"] {
		inputs = append(inputs, []any{v})
	}
	if len(inputs) == 0 {
		t.Fatal("no harvested inputs; the loop cannot be exercised")
	}

	run := func(name, src string) difftest.Result {
		t.Helper()
		res, err := difftest.Run(python, model, "normalize", src, "normalize", inputs)
		if err != nil {
			t.Fatalf("difftest (%s): %v", name, err)
		}
		return res
	}

	// Round 1: the submission. Indexing [0] after .strip() looks fine and
	// passes every plausible hand-written test.
	round1 := run("submission", `
def normalize(s):
    s = s.strip()
    return s[0].upper() + s[1:]
`)
	if round1.Pass() {
		t.Fatal("round 1 found nothing -- hyperray failed to report a real, reachable bug")
	}

	// Round 2: the fix. This is where hyperray must go quiet.
	round2 := run("fixed", `
def normalize(s):
    s = s.strip()
    if not s:
        return ""
    return s[0].upper() + s[1:]
`)
	if !round2.Pass() {
		t.Fatalf("round 2 still reports issues after a correct fix -- false positives make "+
			"the loop unusable: %+v", round2.Disagreements)
	}

	// Round 3: the control. A different bug in the same fixed code must
	// still be caught, or round 2's silence proves nothing.
	round3 := run("control", `
def normalize(s):
    s = s.strip()
    if not s:
        return ""
    return s[0].lower() + s[1:]
`)
	if round3.Pass() {
		t.Fatal("round 3 found nothing either -- round 2's silence was blindness, not a " +
			"clean bill of health, and 'good to go' cannot be trusted")
	}

	t.Logf("round1(submission)=%d issues  round2(fixed)=%d issues  round3(control)=%d issues",
		len(round1.Disagreements), len(round2.Disagreements), len(round3.Disagreements))
}
