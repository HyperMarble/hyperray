package tests

import (
	"testing"

	"github.com/HyperMarble/ray/internal/depharvest"
	"github.com/HyperMarble/ray/internal/difftest"
)

// TestPipeline_HarvestedInputsCatchWhatNaiveTestingMisses is the single
// most load-bearing test in ray, because it is the only one that tests
// ray's actual claim rather than one of its parts.
//
// Every other test here shows a component works. This one asks whether
// the pipeline does something that ordinary testing does not. The
// distinction matters: a diff-test that only "catches" bugs on inputs
// the author hand-picked to hit them proves nothing -- it is a rigged
// demo. The real question is whether inputs ray obtains on its own reach
// branches a plausible hand-written test suite misses.
//
// The bug below is deliberately the kind that survives review: after
// .strip(), indexing [0] is fine for every sensible-looking input and
// raises IndexError only on an empty or whitespace-only string.
//
// Measured result: 5 plausible hand-picked strings find nothing, while
// values harvested from jsonschema's own pinned test suite find it --
// on "                ", a value chosen by that dependency's
// maintainers, not by ray's author.
func TestPipeline_HarvestedInputsCatchWhatNaiveTestingMisses(t *testing.T) {
	python := testPython3(t)
	harvestPython := testHarvestPython(t)
	harvestTests := testHarvestTestsPath(t)

	const model = `
def normalize(s):
    s = s.strip()
    if not s:
        return ""
    return s[0].upper() + s[1:]
`
	const real = `
def normalize(s):
    s = s.strip()
    return s[0].upper() + s[1:]
`

	// What a careful developer would plausibly write by hand.
	naive := [][]any{{"hello"}, {"world"}, {"Test"}, {"abc def"}, {"x"}}
	naiveRes, err := difftest.Run(python, model, "normalize", real, "normalize", naive)
	if err != nil {
		t.Fatalf("difftest (naive): %v", err)
	}
	if !naiveRes.Pass() {
		t.Fatalf("the naive inputs were supposed to MISS this bug; if they now catch it, "+
			"this test no longer demonstrates anything: %+v", naiveRes.Disagreements)
	}

	// Values ray obtains on its own, from a real dependency's real suite.
	h, err := depharvest.Run(harvestPython, "jsonschema", harvestTests, 60000)
	if err != nil {
		t.Fatalf("depharvest: %v", err)
	}
	var harvested [][]any
	for _, v := range h.Values["str"] {
		harvested = append(harvested, []any{v})
	}
	if len(harvested) == 0 {
		t.Fatal("no string values harvested; cannot run the comparison")
	}

	harvestedRes, err := difftest.Run(python, model, "normalize", real, "normalize", harvested)
	if err != nil {
		t.Fatalf("difftest (harvested): %v", err)
	}
	if harvestedRes.Pass() {
		t.Fatalf("harvested inputs found nothing across %d values -- the pipeline did not "+
			"beat naive testing here", harvestedRes.Total)
	}

	// The catch must be the real failure mode, not an unrelated difference.
	found := false
	for _, d := range harvestedRes.Disagreements {
		if d.Model.Outcome == "return" && d.Real.Outcome == "raise" &&
			d.Real.ExceptionType == "IndexError" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("disagreements found, but not the expected IndexError on an empty/blank "+
			"string: %+v", harvestedRes.Disagreements)
	}

	t.Logf("naive: %d inputs, %d found | harvested: %d inputs, %d found",
		naiveRes.Total, len(naiveRes.Disagreements),
		harvestedRes.Total, len(harvestedRes.Disagreements))
}
