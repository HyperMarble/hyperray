package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/ray/internal/specparser"
)

func TestSpecParser_Simple(t *testing.T) {
	content := `# spec.md — Example

## 1. Construction

| n_components | Required behavior |
|---|---|
| 0 | raise ValueError |
| 1+ | construct successfully |
`
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
	tb := tables[0]
	if tb.Section != "1. Construction" {
		t.Errorf("Section = %q", tb.Section)
	}
	wantCols := []string{"n_components", "Required behavior"}
	if len(tb.Columns) != len(wantCols) || tb.Columns[0] != wantCols[0] || tb.Columns[1] != wantCols[1] {
		t.Errorf("Columns = %v, want %v", tb.Columns, wantCols)
	}
	if len(tb.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(tb.Rows))
	}
	if tb.Rows[0][0] != "0" || tb.Rows[0][1] != "raise ValueError" {
		t.Errorf("Rows[0] = %v", tb.Rows[0])
	}
}

func TestSpecParser_MultipleTablesAndSections(t *testing.T) {
	content := `# Title

## 1. First

| a | Required behavior |
|---|---|
| x | y |

Some prose in between.

## 2. Second

| b | c | Required behavior |
|---|---|---|
| p | q | r |
`
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("got %d tables, want 2", len(tables))
	}
	if tables[0].Section != "1. First" || tables[1].Section != "2. Second" {
		t.Errorf("sections = %q, %q", tables[0].Section, tables[1].Section)
	}
	if len(tables[1].Columns) != 3 {
		t.Errorf("second table columns = %v", tables[1].Columns)
	}
}

func TestSpecParser_IgnoresNonTableContent(t *testing.T) {
	content := `# Title

Just prose, no tables here.

- a list
- another item
`
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tables) != 0 {
		t.Fatalf("got %d tables, want 0", len(tables))
	}
}

func TestSpecParser_MismatchedRowLength(t *testing.T) {
	content := `## 1. Broken

| a | b | Required behavior |
|---|---|---|
| x | y |
`
	_, err := specparser.Parse(content)
	if err == nil {
		t.Fatal("Parse: expected an error for a short row, got nil")
	}
}

func TestSpecParser_RealFhplexSpec(t *testing.T) {
	path := filepath.Join("..", "examples", "fhplex-task", "spec.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fhplex spec.md not found: %v", err)
	}
	tables, err := specparser.Parse(string(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tables) < 9 {
		t.Errorf("got %d tables from real spec.md, expected at least 9 (one per numbered section)", len(tables))
	}
	for _, tb := range tables {
		if len(tb.Columns) < 2 {
			t.Errorf("table at line %d (%q) has fewer than 2 columns: %v", tb.Line, tb.Section, tb.Columns)
			continue
		}
		last := strings.ToLower(tb.Columns[len(tb.Columns)-1])
		if !strings.Contains(last, "required behavior") {
			t.Errorf("table at line %d (%q) last column = %q, want it to be a Required-behavior column", tb.Line, tb.Section, tb.Columns[len(tb.Columns)-1])
		}
	}
}
