package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/ray/internal/speclint"
	"github.com/HyperMarble/ray/internal/specparser"
)

func check(t *testing.T, content string) []speclint.Issue {
	t.Helper()
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	issues, err := speclint.Check(tables)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return issues
}

func TestSpecLint_CompleteAndDisjoint(t *testing.T) {
	content := "## 1. Test\n\nParameters: `x` (a / b).\n\n" +
		"| x | Required behavior |\n" +
		"|---|---|\n" +
		"| a | ok a |\n" +
		"| b | ok b |\n"
	issues := check(t, content)
	if len(issues) != 0 {
		t.Fatalf("got %d issues on a complete, disjoint table: %v", len(issues), issues)
	}
}

func TestSpecLint_MissingCombination(t *testing.T) {
	content := "## 1. Test\n\nParameters: `x` (a / b / c), `y` (p / q).\n\n" +
		"| x | y | Required behavior |\n" +
		"|---|---|---|\n" +
		"| a | p | ok |\n" +
		"| a | q | ok |\n" +
		"| b | p | conflict1 |\n" +
		"| b | p | conflict2 |\n" +
		"| c | wrongvalue | ok |\n"
	issues := check(t, content)

	var kinds = map[string]int{}
	for _, iss := range issues {
		kinds[iss.Kind]++
	}
	if kinds["undeclared-value"] != 1 {
		t.Errorf("undeclared-value count = %d, want 1", kinds["undeclared-value"])
	}
	if kinds["disjointness"] != 1 {
		t.Errorf("disjointness count = %d, want 1", kinds["disjointness"])
	}
	if kinds["completeness"] != 3 {
		t.Errorf("completeness count = %d, want 3 (b,q / c,p / c,q missing)", kinds["completeness"])
	}
}

func TestSpecLint_WildcardAndCompound(t *testing.T) {
	content := "## 1. Test\n\nParameters: `x` (a / b), `y` (p / q / r).\n\n" +
		"| x | y | Required behavior |\n" +
		"|---|---|---|\n" +
		"| a | any | covers a |\n" +
		"| b | p, q | covers b-p-q |\n" +
		"| b | r | covers b-r |\n"
	issues := check(t, content)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0 (wildcard + compound should fully cover): %v", len(issues), issues)
	}
}

func TestSpecLint_NotApplicableExcludesColumn(t *testing.T) {
	content := "## 1. Test\n\nParameters: `x` (a / b), `y` (p / q).\n\n" +
		"| x | y | Required behavior |\n" +
		"|---|---|---|\n" +
		"| a | — | covers a regardless of y |\n" +
		"| b | p | covers b-p |\n" +
		"| b | q | covers b-q |\n"
	issues := check(t, content)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0: %v", len(issues), issues)
	}
}

func TestSpecLint_BacktickInsideValueIsStripped(t *testing.T) {
	content := "## 1. Test\n\nParameters: `x` (mock / real `NaiveForecaster`).\n\n" +
		"| x | Required behavior |\n" +
		"|---|---|\n" +
		"| mock | ok |\n" +
		"| real NaiveForecaster | ok |\n"
	issues := check(t, content)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0 (mid-value backtick should be stripped): %v", len(issues), issues)
	}
}

func TestSpecLint_RealFhplexSpec(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "examples", "fhplex-task", "spec.md"))
	if err != nil {
		t.Skipf("fhplex spec.md not found: %v", err)
	}
	issues := check(t, string(content))
	if len(issues) != 0 {
		t.Fatalf("got %d issues on the real fhplex spec.md: %v", len(issues), issues)
	}
}
