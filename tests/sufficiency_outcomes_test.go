package tests

import (
	"testing"

	"github.com/HyperMarble/hyperray/internal/sufficiency"
)

// TestSufficiency_ExtractOutcomes_UniformRaiseAndReturn is the
// regression test for extract_outcomes.py: raises and returns are
// extracted uniformly by one script, not two separate heuristics, and a
// return's value is captured as its real source text regardless of
// shape (literal, bare name, or computed expression) -- no per-shape
// special-casing. Output is sorted by source line -- ast.walk() is
// breadth-first, not source order, which a real run against this exact
// test case caught before the extractor sorted its output.
func TestSufficiency_ExtractOutcomes_UniformRaiseAndReturn(t *testing.T) {
	python := testPython3(t)
	src := writeTempPy(t, `
def f(n, name):
    if n == 0:
        raise ValueError("at least one component")
    if n < 0:
        return False
    if n == 1:
        return name
    return n + 1
`)
	outcomes, err := sufficiency.ExtractOutcomes(python, extractScriptPath(t), src, sufficiency.LangPython)
	if err != nil {
		t.Fatalf("ExtractOutcomes: %v", err)
	}
	if len(outcomes) != 4 {
		t.Fatalf("got %d outcomes, want 4: %+v", len(outcomes), outcomes)
	}

	if outcomes[0].Kind != "raise" ||
		outcomes[0].SourceText != `raise ValueError("at least one component")` {
		t.Errorf("outcome 0 = %+v, want the verbatim raise text", outcomes[0])
	}
	// SourceText is the verbatim full statement, not just the returned
	// expression -- that keeps the extractor language-agnostic (the same
	// rule produces `throw std::invalid_argument(...)` for C++ and
	// `return 0, errors.New(...)` for Go) with no per-language knowledge
	// of which child node holds the value.
	if outcomes[1].Kind != "return" || outcomes[1].SourceText != "return False" {
		t.Errorf("outcome 1 = %+v, want return \"return False\" (literal)", outcomes[1])
	}
	if outcomes[2].Kind != "return" || outcomes[2].SourceText != "return name" {
		t.Errorf("outcome 2 = %+v, want return \"return name\" (bare name)", outcomes[2])
	}
	if outcomes[3].Kind != "return" || outcomes[3].SourceText != "return n + 1" {
		t.Errorf("outcome 3 = %+v, want return \"return n + 1\" (computed expression)", outcomes[3])
	}
}

// TestSufficiency_ExtractOutcomes_BareReturnOmitted confirms a bare
// `return` (implicit None) is skipped -- it isn't a distinct outcome
// worth matching against spec.md.
func TestSufficiency_ExtractOutcomes_BareReturnOmitted(t *testing.T) {
	python := testPython3(t)
	src := writeTempPy(t, `
def f(n):
    if n == 0:
        return
    return n
`)
	outcomes, err := sufficiency.ExtractOutcomes(python, extractScriptPath(t), src, sufficiency.LangPython)
	if err != nil {
		t.Fatalf("ExtractOutcomes: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("got %d outcomes, want 1 (bare return should be skipped): %+v", len(outcomes), outcomes)
	}
	if outcomes[0].SourceText != "return n" {
		t.Errorf("outcome 0 = %+v, want return \"return n\"", outcomes[0])
	}
}
