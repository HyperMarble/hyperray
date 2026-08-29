// Package speclint checks specparser.Table condition tables for
// completeness and disjointness against their declared Parameters domain.
package speclint

import (
	"fmt"
	"strings"

	"github.com/HyperMarble/hyperray/internal/specparser"
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
	paramCols := tb.ParamColumns()

	if tb.Params == "" {
		// No declared domain to check against; nothing to lint.
		return nil, nil
	}

	domains, unsupported, err := specparser.ParseParams(tb.Params)
	if err != nil {
		return nil, err
	}
	if unsupported != "" {
		return []Issue{{
			Section: tb.Section,
			Line:    tb.Line,
			Kind:    "unsupported-domain",
			Message: fmt.Sprintf("parameter %q has neither a quoted singleton nor a ` / `-separated finite value list — decompose it into explicit categorical buckets, not a numeric/continuous range", unsupported),
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
		total *= len(d.Values)
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
				rowSets[r][c] = domains[c].Values
				continue
			}
			values, _, parseErr := specparser.ParseValueList(cell)
			if parseErr != nil {
				return nil, fmt.Errorf("column %q row value: %w", domains[c].Name, parseErr)
			}
			var set []string
			for _, tok := range values {
				if !contains(domains[c].Values, tok) {
					issues = append(issues, Issue{
						Section: tb.Section,
						Line:    tb.Line,
						Kind:    "undeclared-value",
						Message: fmt.Sprintf("column %q uses value %q, not declared in Parameters", domains[c].Name, tok),
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
		valueDomains[c] = domains[c].Values
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
