package tests

import (
	"testing"

	"github.com/HyperMarble/hyperray/internal/difftest"
)

const clampModel = `
def clamp(x, lo, hi):
    if x < lo: return lo
    if x > hi: return hi
    return x
`

func gridInputs() [][]any {
	var in [][]any
	for _, x := range []any{-5, -1, 0, 1, 5, 10, 11} {
		in = append(in, []any{x, 0, 10})
	}
	return in
}

// TestDiffTest_AgreesWhenImplementationMatches is the baseline: a real
// implementation that differs only in style from the proven model must
// produce zero disagreements.
func TestDiffTest_AgreesWhenImplementationMatches(t *testing.T) {
	python := testPython3(t)
	real := `
def clamp(x, lo, hi):
    return max(lo, min(x, hi))
`
	res, err := difftest.Run(python, clampModel, "clamp", real, "clamp", gridInputs())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Pass() {
		t.Fatalf("want agreement, got %d disagreements: %+v", len(res.Disagreements), res.Disagreements)
	}
	if res.Agreements != res.Total || res.Total != len(gridInputs()) {
		t.Fatalf("got %d/%d agreements over %d inputs", res.Agreements, res.Total, len(gridInputs()))
	}
}

// TestDiffTest_CatchesOffByOne is the case this whole layer exists for:
// the model is proven correct, the real implementation has a subtle
// off-by-one, and only running both on concrete inputs reveals it. The
// proof alone would never have caught this, because the proof is about
// the model, not about the shipped code.
func TestDiffTest_CatchesOffByOne(t *testing.T) {
	python := testPython3(t)
	real := `
def clamp(x, lo, hi):
    if x < lo: return lo
    if x > hi: return hi - 1   # off-by-one: silently wrong only above hi
    return x
`
	res, err := difftest.Run(python, clampModel, "clamp", real, "clamp", gridInputs())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Pass() {
		t.Fatal("the off-by-one was not caught -- diff-test is not doing its job")
	}
	// x=11 is the only input above hi, so exactly one input should differ.
	if len(res.Disagreements) != 1 {
		t.Fatalf("want exactly 1 disagreement (x=11), got %d: %+v",
			len(res.Disagreements), res.Disagreements)
	}
	d := res.Disagreements[0]
	if d.Model.Outcome != "return" || d.Real.Outcome != "return" {
		t.Errorf("both sides should have returned: %+v", d)
	}
}

// TestDiffTest_ExceptionTypeMismatchIsADisagreement confirms raising the
// wrong exception type counts as drift, not as agreement.
func TestDiffTest_ExceptionTypeMismatchIsADisagreement(t *testing.T) {
	python := testPython3(t)
	model := `
def f(n):
    if n == 0: raise ValueError("at least one")
    return n
`
	real := `
def f(n):
    if n == 0: raise TypeError("at least one")
    return n
`
	res, err := difftest.Run(python, model, "f", real, "f", [][]any{{0}, {1}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Disagreements) != 1 {
		t.Fatalf("want 1 disagreement (ValueError vs TypeError), got %d: %+v",
			len(res.Disagreements), res.Disagreements)
	}
	d := res.Disagreements[0]
	if d.Model.ExceptionType != "ValueError" || d.Real.ExceptionType != "TypeError" {
		t.Errorf("got model=%q real=%q, want ValueError vs TypeError",
			d.Model.ExceptionType, d.Real.ExceptionType)
	}
}

// TestDiffTest_SameExceptionDifferentMessageAgrees encodes a deliberate
// decision: spec.md states required behaviour, not exact wording, so two
// correct implementations may word a message differently. Only the
// exception TYPE is compared.
func TestDiffTest_SameExceptionDifferentMessageAgrees(t *testing.T) {
	python := testPython3(t)
	model := `
def f(n):
    if n == 0: raise ValueError("at least one component")
    return n
`
	real := `
def f(n):
    if n == 0: raise ValueError("need >= 1 component")
    return n
`
	res, err := difftest.Run(python, model, "f", real, "f", [][]any{{0}, {1}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Pass() {
		t.Fatalf("differing messages should agree, got: %+v", res.Disagreements)
	}
}

// TestDiffTest_ReturnVsRaiseIsADisagreement confirms the asymmetric case:
// one side returning while the other raises is always drift.
func TestDiffTest_ReturnVsRaiseIsADisagreement(t *testing.T) {
	python := testPython3(t)
	model := `
def f(n):
    if n < 0: raise ValueError("negative")
    return n
`
	real := `
def f(n):
    if n < 0: return 0   # swallows the error instead of raising
    return n
`
	res, err := difftest.Run(python, model, "f", real, "f", [][]any{{-1}, {1}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Disagreements) != 1 {
		t.Fatalf("want 1 disagreement, got %d: %+v", len(res.Disagreements), res.Disagreements)
	}
	d := res.Disagreements[0]
	if d.Model.Outcome != "raise" || d.Real.Outcome != "return" {
		t.Errorf("got model=%s real=%s, want raise vs return", d.Model.Outcome, d.Real.Outcome)
	}
}

// TestDiffTest_NamespacesAreIsolated confirms a helper defined only in
// one side cannot satisfy a missing name in the other -- that would mask
// a real difference behind a shared namespace.
func TestDiffTest_NamespacesAreIsolated(t *testing.T) {
	python := testPython3(t)
	model := `
def _helper(x): return x * 2
def f(n): return _helper(n)
`
	real := `
def f(n): return _helper(n)   # _helper is NOT defined here
`
	res, err := difftest.Run(python, model, "f", real, "f", [][]any{{3}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Disagreements) != 1 {
		t.Fatalf("want 1 disagreement (NameError in real), got %+v", res.Disagreements)
	}
	if res.Disagreements[0].Real.ExceptionType != "NameError" {
		t.Errorf("got %q, want NameError -- namespaces are leaking",
			res.Disagreements[0].Real.ExceptionType)
	}
}
