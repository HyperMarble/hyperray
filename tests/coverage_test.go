package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/internal/coverage"
	"github.com/HyperMarble/hyperray/internal/specparser"
)

// testPictPath locates a pict binary for tests: RAY_PICT_PATH env var
// first, else whatever "pict" resolves to on PATH. Skips the calling
// test if neither is available — pict isn't bundled yet (v0.1.0).
func testPictPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("RAY_PICT_PATH"); p != "" {
		return p
	}
	if p, err := exec.LookPath("pict"); err == nil {
		return p
	}
	t.Skip("pict binary not found; set RAY_PICT_PATH to test coverage")
	return ""
}

func TestCoverage_SimpleModel(t *testing.T) {
	pict := testPictPath(t)
	content := "## 1. Test\n\nParameters: `x` (a / b), `y` (p / q).\n\n" +
		"| x | y | Required behavior |\n" +
		"|---|---|---|\n" +
		"| a | p | ok |\n" +
		"| a | q | ok |\n" +
		"| b | p | ok |\n" +
		"| b | q | ok |\n"
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	results, err := coverage.Generate(tables, pict, 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d table results, want 1", len(results))
	}
	// 2 params x 2 values each, pairwise strength = full 2x2 = 4 combos.
	if len(results[0].Combinations) != 4 {
		t.Fatalf("got %d combinations, want 4", len(results[0].Combinations))
	}
	seen := map[string]bool{}
	for _, c := range results[0].Combinations {
		if c["x"] == "" || c["y"] == "" {
			t.Errorf("combination missing a key: %v", c)
		}
		seen[c["x"]+"|"+c["y"]] = true
	}
	for _, want := range []string{"a|p", "a|q", "b|p", "b|q"} {
		if !seen[want] {
			t.Errorf("missing expected combination %q", want)
		}
	}
}

func TestCoverage_ValueWithCommaSurvives(t *testing.T) {
	pict := testPictPath(t)
	content := "## 1. Test\n\nParameters: `x` (wildcard, all values / plain).\n\n" +
		"| x | Required behavior |\n" +
		"|---|---|\n" +
		"| wildcard, all values | ok |\n" +
		"| plain | ok |\n"
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	results, err := coverage.Generate(tables, pict, 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	found := false
	for _, c := range results[0].Combinations {
		if c["x"] == "wildcard, all values" {
			found = true
		}
	}
	if !found {
		t.Fatalf("value containing a comma was not preserved intact: %v", results[0].Combinations)
	}
}

func TestCoverage_SkipsTablesWithoutParams(t *testing.T) {
	pict := testPictPath(t)
	content := "## 1. Test\n\n| Property | Required behavior |\n|---|---|\n| clonability | ok |\n"
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	results, err := coverage.Generate(tables, pict, 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0 (table has no Parameters line)", len(results))
	}
}

func TestCoverage_RealFhplexSpec(t *testing.T) {
	pict := testPictPath(t)
	content, err := os.ReadFile(filepath.Join("..", "examples", "fhplex-task", "spec.md"))
	if err != nil {
		t.Skipf("fhplex spec.md not found: %v", err)
	}
	tables, err := specparser.Parse(string(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	results, err := coverage.Generate(tables, pict, 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("got 0 table results on the real fhplex spec.md")
	}
	for _, r := range results {
		if len(r.Combinations) == 0 {
			t.Errorf("table %q (line %d) produced 0 combinations", r.Section, r.Line)
		}
	}
}
