package tests

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestSpecParser_QuotedFiniteValuesRoundTrip(t *testing.T) {
	raw := "Parameters: `resource` (" +
		`"/api/v1" / "https://example/x/y?q=1/2" / "2026/08/27" / "left / right" / "quote: \" and slash: \\" / "雪/☃"` +
		")."
	domains, unsupported, err := specparser.ParseParams(raw)
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if unsupported != "" || len(domains) != 1 {
		t.Fatalf("domains=%+v unsupported=%q", domains, unsupported)
	}
	want := []string{
		"/api/v1",
		"https://example/x/y?q=1/2",
		"2026/08/27",
		"left / right",
		"quote: \" and slash: \\",
		"雪/☃",
	}
	if !reflect.DeepEqual(domains[0].Values, want) {
		t.Fatalf("values=%q, want %q", domains[0].Values, want)
	}
	for _, value := range want {
		if !domains[0].JSONQuoted[value] {
			t.Errorf("value %q lost its JSON-quoted boundary", value)
		}
	}

	compound, separated, err := specparser.ParseValueList(`"a/b" / "c/d"`)
	if err != nil || !separated || !reflect.DeepEqual(compound, []string{"a/b", "c/d"}) {
		t.Fatalf("compound=%q separated=%v err=%v", compound, separated, err)
	}
}

func TestSpecParser_QuotedSingletonDomain(t *testing.T) {
	domains, unsupported, err := specparser.ParseParams("Parameters: `route` (\"/api/v1\").")
	if err != nil || unsupported != "" || len(domains) != 1 || !reflect.DeepEqual(domains[0].Values, []string{"/api/v1"}) {
		t.Fatalf("quoted singleton domains=%+v unsupported=%q err=%v", domains, unsupported, err)
	}
	bare, unsupported, err := specparser.ParseParams("Parameters: `route` (only).")
	if err != nil || len(bare) != 0 || unsupported != "route" {
		t.Fatalf("ambiguous bare singleton domains=%+v unsupported=%q err=%v", bare, unsupported, err)
	}
}

func TestSpecParser_QuotedAnyDiffersFromWildcard(t *testing.T) {
	domain := specparser.Domain{Name: "mode", Values: []string{"any", "other"}, JSONQuoted: map[string]bool{"any": true}}
	if got := specparser.CellValues(`"any"`, domain); !reflect.DeepEqual(got, []string{"any"}) {
		t.Fatalf("quoted any values=%q, want literal any", got)
	}
	if got := specparser.CellValues("any", domain); !reflect.DeepEqual(got, domain.Values) {
		t.Fatalf("unquoted any values=%q, want wildcard %q", got, domain.Values)
	}
}

func TestSpecParser_RejectsAmbiguousFiniteValues(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  string
	}{
		{name: "unquoted path", raw: `/api/v1 / other`},
		{name: "unquoted URL", raw: `https://example/x/y?q=1/2 / other`},
		{name: "unterminated quote", raw: `"/api/v1 / other`},
		{name: "invalid JSON escape", raw: `"bad\q" / other`},
		{name: "mixed quoted and bare token", raw: `"quoted"suffix / other`},
		{name: "separator without spaces", raw: `a/b`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if values, _, err := specparser.ParseValueList(test.raw); err == nil {
				t.Fatalf("ParseValueList(%q)=%q, want strict rejection", test.raw, values)
			}
		})
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

// TestSpecParser_TrailingProseInParametersParagraphIsIgnored is the
// regression test for a real bug two independent Layer-3 stress tests
// found: when the Parameters: sentence shares a Markdown paragraph with
// trailing explanatory prose (no blank line between them), the whole
// paragraph was being scanned for domain declarations -- so a backtick
// or a "/" inside that trailing prose got misread as more parameters,
// even though the author did nothing wrong. The fix truncates parsing
// at the declaration sentence's own real end (its first top-level
// period), so trailing prose is out of scope entirely.
func TestSpecParser_TrailingProseInParametersParagraphIsIgnored(t *testing.T) {
	content := "## 1. Test\n\n" +
		"Parameters: `x` (a / b). This means `a` is the fast path (see docs/spec) " +
		"or `b` is the slow path/legacy.\n\n" +
		"| x | Required behavior |\n|---|---|\n| a | ok |\n| b | ok |\n"
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
	doms, unsupported, err := specparser.ParseParams(tables[0].Params)
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if unsupported != "" {
		t.Fatalf("got unsupported domain %q, want none (trailing prose should be ignored)", unsupported)
	}
	if len(doms) != 1 {
		t.Fatalf("got %d domains, want 1 (trailing prose's backticks should not become extra parameters): %+v", len(doms), doms)
	}
	if doms[0].Name != "x" || len(doms[0].Values) != 2 || doms[0].Values[0] != "a" || doms[0].Values[1] != "b" {
		t.Fatalf("got domain %+v, want x: [a b]", doms[0])
	}
}

// TestSpecParser_LeadInProseBeforeParametersIsNotLost is the regression
// test for a real, more dangerous bug found while stress-testing the fix
// above: a lead-in sentence sharing the same paragraph as "Parameters:"
// (no blank line before it) made the domain declaration silently
// disappear entirely -- not corrupt, gone -- because the paragraph no
// longer STARTED WITH "Parameters:". With no domain to check against,
// ray spec-lint reported a clean PASS on a table containing a genuinely
// undeclared cell value. Matching on Contains instead of HasPrefix keeps
// the declaration regardless of what precedes it in the paragraph.
func TestSpecParser_LeadInProseBeforeParametersIsNotLost(t *testing.T) {
	content := "## 1. Test\n\n" +
		"See the note above about `foo`. Parameters: `x` (a / b).\n\n" +
		"| x | Required behavior |\n|---|---|\n| a | ok |\n| b | ok |\n"
	tables, err := specparser.Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
	if tables[0].Params == "" {
		t.Fatal("Params is empty -- the domain declaration was lost because of the lead-in sentence")
	}
	doms, unsupported, err := specparser.ParseParams(tables[0].Params)
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if unsupported != "" {
		t.Fatalf("got unsupported domain %q, want none", unsupported)
	}
	if len(doms) != 1 || doms[0].Name != "x" || len(doms[0].Values) != 2 {
		t.Fatalf("got domains %+v, want one domain x: [a b]", doms)
	}
}
