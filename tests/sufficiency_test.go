package tests

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/specparser"
	"github.com/HyperMarble/hyperray/internal/sufficiency"
)

func testPython3(t *testing.T) string {
	t.Helper()
	if p, err := exec.LookPath("python3"); err == nil {
		return p
	}
	t.Skip("python3 not found on PATH")
	return ""
}

func extractScriptPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "third_party", "branch-extract", "extract_outcomes.py"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeTempPy(t *testing.T, content string) string {
	t.Helper()
	return writeTempFile(t, "sufficiency-src-*.py", content)
}

func onlyRaises(outcomes []sufficiency.Outcome) []sufficiency.Outcome {
	var raises []sufficiency.Outcome
	for _, o := range outcomes {
		if o.Kind == "raise" {
			raises = append(raises, o)
		}
	}
	return raises
}

// TestSufficiency_ExtractOutcomes_Raises covers raise extraction, and
// the fact that a bare `raise` re-raise is excluded by the query itself
// (via the grammar requiring a child expression) rather than by any
// Python-side filtering rule of hyperray's own.
func TestSufficiency_ExtractOutcomes_Raises(t *testing.T) {
	python := testPython3(t)
	src := writeTempPy(t, `
def f(n):
    if n == 0:
        raise ValueError("at least one component")
    if n < 0:
        raise TypeError("must be probability distributions")
    try:
        pass
    except Exception:
        raise  # bare re-raise -- excluded by the grammar, no text to match
    return n
`)
	outcomes, err := sufficiency.ExtractOutcomes(python, extractScriptPath(t), src, sufficiency.LangPython)
	if err != nil {
		t.Fatalf("ExtractOutcomes: %v", err)
	}
	raises := onlyRaises(outcomes)
	if len(raises) != 2 {
		t.Fatalf("got %d raises, want 2 (bare re-raise should be excluded): %+v", len(raises), raises)
	}
	if !strings.Contains(raises[0].SourceText, "ValueError") ||
		!strings.Contains(raises[0].SourceText, "at least one component") {
		t.Errorf("raise 0 = %+v, want verbatim text of the ValueError raise", raises[0])
	}
	if !strings.Contains(raises[1].SourceText, "TypeError") ||
		!strings.Contains(raises[1].SourceText, "must be probability distributions") {
		t.Errorf("raise 1 = %+v, want verbatim text of the TypeError raise", raises[1])
	}
}

// TestSufficiency_InterpolatedMessageHandledByVerbatimText documents a
// real simplification: an earlier version detected interpolated messages
// with a hand-written list of node types so it could omit them. Matching
// verbatim source text makes that unnecessary -- an interpolated raise
// keeps whatever literal fragments it has, so spec.md can still match on
// them, and no special case is needed.
func TestSufficiency_InterpolatedMessageHandledByVerbatimText(t *testing.T) {
	python := testPython3(t)
	src := writeTempPy(t, `
def f(name):
    raise ValueError(f"unknown key {name}")
`)
	outcomes, err := sufficiency.ExtractOutcomes(python, extractScriptPath(t), src, sufficiency.LangPython)
	if err != nil {
		t.Fatalf("ExtractOutcomes: %v", err)
	}
	raises := onlyRaises(outcomes)
	if len(raises) != 1 {
		t.Fatalf("got %d raises, want 1", len(raises))
	}
	if !strings.Contains(raises[0].SourceText, "unknown key") {
		t.Errorf("got %+v, want the literal fragment preserved in verbatim text", raises[0])
	}

	// spec.md quoting the literal fragment covers it, with no
	// interpolation-detection rule anywhere in the pipeline.
	content := "## 1. Test\n\nParameters: `name` (present / absent).\n\n" +
		"| name | Required behavior |\n|---|---|\n" +
		"| present | raise ValueError containing \"unknown key\" |\n" +
		"| absent | raise ValueError containing \"unknown key\" |\n"
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if gaps := sufficiency.CheckRaiseCoverage(tables, outcomes); len(gaps) != 0 {
		t.Fatalf("got %d gaps, want 0: %+v", len(gaps), gaps)
	}
}

// TestSufficiency_CheckRaiseCoverage_CleanSpec confirms a spec.md that
// quotes every real raise's message reports zero gaps.
func TestSufficiency_CheckRaiseCoverage_CleanSpec(t *testing.T) {
	python := testPython3(t)
	src := writeTempPy(t, `
def f(n):
    if n == 0:
        raise ValueError("at least one component")
    return n
`)
	outcomes, err := sufficiency.ExtractOutcomes(python, extractScriptPath(t), src, sufficiency.LangPython)
	if err != nil {
		t.Fatalf("ExtractOutcomes: %v", err)
	}
	content := "## 1. Test\n\nParameters: `n` (0 / nonzero).\n\n" +
		"| n | Required behavior |\n|---|---|\n" +
		"| 0 | raise ValueError containing \"at least one component\" |\n" +
		"| nonzero | returns n |\n"
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	gaps := sufficiency.CheckRaiseCoverage(tables, outcomes)
	if len(gaps) != 0 {
		t.Fatalf("got %d gaps, want 0: %+v", len(gaps), gaps)
	}
}

// TestSufficiency_CheckRaiseCoverage_RealGap confirms a real raise
// spec.md never mentions gets reported -- the actual sufficiency check
// this package exists for, and the same shape as the four real gaps
// found against the actual sktime RowwiseDistribution source.
func TestSufficiency_CheckRaiseCoverage_RealGap(t *testing.T) {
	python := testPython3(t)
	src := writeTempPy(t, `
def f(n):
    if n == 0:
        raise ValueError("at least one component")
    if n < 0:
        raise TypeError("must be probability distributions")
    return n
`)
	outcomes, err := sufficiency.ExtractOutcomes(python, extractScriptPath(t), src, sufficiency.LangPython)
	if err != nil {
		t.Fatalf("ExtractOutcomes: %v", err)
	}
	// spec.md only covers the ValueError case -- the TypeError case is a
	// real, undeclared gap.
	content := "## 1. Test\n\nParameters: `n` (0 / nonzero).\n\n" +
		"| n | Required behavior |\n|---|---|\n" +
		"| 0 | raise ValueError containing \"at least one component\" |\n" +
		"| nonzero | returns n |\n"
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	gaps := sufficiency.CheckRaiseCoverage(tables, outcomes)
	if len(gaps) != 1 {
		t.Fatalf("got %d gaps, want 1 (the undeclared TypeError): %+v", len(gaps), gaps)
	}
	if !strings.Contains(gaps[0].SourceText, "TypeError") {
		t.Errorf("gap = %+v, want the TypeError raise", gaps[0])
	}
}
