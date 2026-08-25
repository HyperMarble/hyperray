// Package speclint checks specparser.Table condition tables for
// completeness and disjointness against their declared Parameters domain.
package speclint

import (
	"fmt"
	"strings"

	"github.com/HyperMarble/ray/internal/specparser"
)

type Issue struct {
	Section string `json:"section"`
	Line    int    `json:"line"`
	Kind    string `json:"kind"` // "completeness", "disjointness", "undeclared-value", "schema-mismatch", "unsupported-domain"
	Message string `json:"message"`
}

const maxCombinations = 10000

func isWildcard(cell string) bool {
	c := strings.ToLower(strings.TrimSpace(cell))
	return c == "any" || c == "—" || c == "-" || c == ""
}

type domain struct {
	name   string
	values []string
}

func Check(tables []specparser.Table) ([]Issue, error) {
	var issues []Issue
	for _, tb := range tables {
		tblIssues, err := checkTable(tb)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", tb.Section, err)
		}
		issues = append(issues, tblIssues...)
	}
	return issues, nil
}

func checkTable(tb specparser.Table) ([]Issue, error) {
	if len(tb.Columns) < 2 || len(tb.Rows) == 0 {
		return nil, nil
	}
	paramCols := len(tb.Columns) - 1

	if tb.Params == "" {
		// No declared domain to check against; nothing to lint.
		return nil, nil
	}

	domains, unsupported, err := parseParams(tb.Params)
	if err != nil {
		return nil, err
	}
	if unsupported != "" {
		return []Issue{{
			Section: tb.Section,
			Line:    tb.Line,
			Kind:    "unsupported-domain",
			Message: fmt.Sprintf("parameter %q has no declared value list (ordered/range domains are not yet supported)", unsupported),
		}}, nil
	}
	if len(domains) != paramCols {
		return []Issue{{
			Section: tb.Section,
			Line:    tb.Line,
			Kind:    "schema-mismatch",
			Message: fmt.Sprintf("declares %d parameters but table has %d parameter columns", len(domains), paramCols),
		}}, nil
	}

	var issues []Issue

	total := 1
	for _, d := range domains {
		total *= len(d.values)
		if total > maxCombinations {
			return nil, fmt.Errorf("too many combinations to check (>%d)", maxCombinations)
		}
	}

	rowSets := make([][][]string, len(tb.Rows)) // rowSets[r][c] = set of values row r allows in column c
	for r, row := range tb.Rows {
		rowSets[r] = make([][]string, paramCols)
		for c := 0; c < paramCols; c++ {
			cell := row[c]
			if isWildcard(cell) {
				rowSets[r][c] = domains[c].values
				continue
			}
			var set []string
			for _, tok := range strings.Split(cell, "/") {
				tok = strings.TrimSpace(strings.ReplaceAll(tok, "`", ""))
				if tok == "" {
					continue
				}
				if !contains(domains[c].values, tok) {
					issues = append(issues, Issue{
						Section: tb.Section,
						Line:    tb.Line,
						Kind:    "undeclared-value",
						Message: fmt.Sprintf("column %q uses value %q, not declared in Parameters", domains[c].name, tok),
					})
					continue
				}
				set = append(set, tok)
			}
			rowSets[r][c] = set
		}
	}

	valueDomains := make([][]string, paramCols)
	for c := range valueDomains {
		valueDomains[c] = domains[c].values
	}
	combos := cartesian(valueDomains)

	for _, combo := range combos {
		var matches []int
		for r := range tb.Rows {
			if comboMatchesRow(combo, rowSets[r]) {
				matches = append(matches, r)
			}
		}
		if len(matches) == 0 {
			issues = append(issues, Issue{
				Section: tb.Section,
				Line:    tb.Line,
				Kind:    "completeness",
				Message: fmt.Sprintf("combination (%s) has no matching row", strings.Join(combo, ", ")),
			})
			continue
		}
		behaviors := map[string]bool{}
		for _, r := range matches {
			behaviors[tb.Rows[r][paramCols]] = true
		}
		if len(behaviors) > 1 {
			issues = append(issues, Issue{
				Section: tb.Section,
				Line:    tb.Line,
				Kind:    "disjointness",
				Message: fmt.Sprintf("combination (%s) matches conflicting rows with different required behavior", strings.Join(combo, ", ")),
			})
		}
	}
	return issues, nil
}

func comboMatchesRow(combo []string, rowSets [][]string) bool {
	for c, v := range combo {
		if !contains(rowSets[c], v) {
			return false
		}
	}
	return true
}

func contains(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

// parseParams parses "Parameters: `name` (v1 / v2), `name2` (v3 / v4)."
// into one domain per parameter, in declared order. If a parameter's
// parenthesized group contains no "/", its domain can't be enumerated
// (an ordered/range domain, not yet supported) and its name is returned
// as unsupported.
func parseParams(raw string) (doms []domain, unsupported string, err error) {
	s := raw
	if idx := strings.Index(s, "Parameters:"); idx >= 0 {
		s = s[idx+len("Parameters:"):]
	}

	i := 0
	for i < len(s) {
		// Find next backtick-quoted name.
		start := strings.IndexByte(s[i:], '`')
		if start < 0 {
			break
		}
		start += i
		end := strings.IndexByte(s[start+1:], '`')
		if end < 0 {
			return nil, "", fmt.Errorf("unterminated backtick in Parameters line")
		}
		end += start + 1
		name := s[start+1 : end]

		// Find the opening paren after the name.
		rest := s[end+1:]
		parenStart := strings.IndexByte(rest, '(')
		if parenStart < 0 {
			i = end + 1
			continue
		}
		depth := 0
		parenEnd := -1
		for p := parenStart; p < len(rest); p++ {
			switch rest[p] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					parenEnd = p
				}
			}
			if parenEnd >= 0 {
				break
			}
		}
		if parenEnd < 0 {
			return nil, "", fmt.Errorf("unterminated parenthesis for parameter %q", name)
		}
		content := rest[parenStart+1 : parenEnd]
		if idx := strings.Index(content, "—"); idx >= 0 {
			content = content[:idx]
		}

		if !strings.Contains(content, "/") {
			return nil, name, nil
		}

		var values []string
		for _, v := range strings.Split(content, "/") {
			v = strings.TrimSpace(v)
			v = strings.ReplaceAll(v, "`", "")
			if v != "" {
				values = append(values, v)
			}
		}
		doms = append(doms, domain{name: name, values: values})

		i = end + 1 + parenEnd + 1
	}
	return doms, "", nil
}

func cartesian(domains [][]string) [][]string {
	result := [][]string{{}}
	for _, d := range domains {
		var next [][]string
		for _, prefix := range result {
			for _, v := range d {
				combo := append(append([]string{}, prefix...), v)
				next = append(next, combo)
			}
		}
		result = next
	}
	return result
}
