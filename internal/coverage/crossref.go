package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/internal/specparser"
)

// This file is layer 2's actual job. Generating the combination matrix
// was only ever half of it -- the design has always said the layer
// "flags any required combination with zero matching test" -- and until
// now nothing here read the tests at all, while the pass still rendered
// as a tick.
//
// A combination with no test is the false positive this whole tool
// exists to prevent: the requirement is stated, nothing checks it, and a
// solver that skips it passes anyway.

// Status is what is known about one combination.
type Status int

const (
	// Declared: a row covers the combination and names a test that exists.
	// Not proof the test enforces it -- see internal/enforce -- but the
	// author has committed to a specific claim hyperray can go check.
	Declared Status = iota
	// Unenforced: a row covers the combination and names no test.
	Unenforced
	// Missing: the named test does not appear in the test sources.
	Missing
	// Uncovered: no row covers the combination at all. spec-lint reports
	// this as a completeness failure; repeated here because a combination
	// with no row also has no test.
	Uncovered
)

// Finding is one combination that no test is known to enforce.
type Finding struct {
	Section string      `json:"section"`
	Line    int         `json:"line"`
	Combo   Combination `json:"combination"`
	Status  Status      `json:"status"`
	Test    string      `json:"test,omitempty"`
}

func (f Finding) String() string {
	var parts []string
	for k, v := range f.Combo {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	combo := strings.Join(parts, " ")
	switch f.Status {
	case Unenforced:
		return fmt.Sprintf("%s: %s — no test declared", f.Section, combo)
	case Missing:
		return fmt.Sprintf("%s: %s — declares %q, which no test file defines", f.Section, combo, f.Test)
	case Uncovered:
		return fmt.Sprintf("%s: %s — no row covers this combination", f.Section, combo)
	}
	return fmt.Sprintf("%s: %s", f.Section, combo)
}

// testNamePattern matches a test declaration in the languages hyperray
// targets. Only definitions count: a test's own name appearing inside
// another test's body must not make it look defined.
var testNamePattern = regexp.MustCompile(
	`(?m)(?:^\s*(?:async\s+)?def\s+([A-Za-z_][A-Za-z_0-9]*)` + // python
		`|^\s*func\s+(Test[A-Za-z_0-9]*)` + // go
		`|^\s*(?:pub\s+)?(?:async\s+)?fn\s+([A-Za-z_][A-Za-z_0-9]*)` + // rust
		`|^\s*(?:TEST|TEST_F|TEST_P)\s*\(\s*([A-Za-z_0-9]+)\s*,\s*([A-Za-z_0-9]+)\s*\)` + // gtest
		`|(?:it|test)\s*\(\s*['"` + "`" + `]([^'"` + "`" + `]{1,200})['"` + "`" + `])`) // js/ts

// TestNames collects every test defined under dir, at any depth.
func TestNames(dir string) (map[string]bool, error) {
	names := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, m := range testNamePattern.FindAllStringSubmatch(string(body), -1) {
			for _, group := range m[1:] {
				if group != "" {
					names[group] = true
				}
			}
		}
		return nil
	})
	return names, err
}

// CrossReference pairs every generated combination with the row that
// covers it and reports the ones no test is known to enforce.
func CrossReference(tables []specparser.Table, covs []TableCoverage, testNames map[string]bool) []Finding {
	bySection := map[string]specparser.Table{}
	for _, tb := range tables {
		bySection[tb.Section] = tb
	}

	var findings []Finding
	for _, cov := range covs {
		tb, ok := bySection[cov.Section]
		if !ok {
			continue
		}
		domains, _, err := specparser.ParseParams(tb.Params)
		if err != nil {
			continue
		}
		for _, combo := range cov.Combinations {
			row, found := rowFor(tb, domains, combo)
			if !found {
				findings = append(findings, Finding{cov.Section, cov.Line, combo, Uncovered, ""})
				continue
			}
			test := tb.EnforcedBy(row)
			switch {
			case test == "":
				findings = append(findings, Finding{cov.Section, cov.Line, combo, Unenforced, ""})
			case !namedTestExists(test, testNames):
				findings = append(findings, Finding{cov.Section, cov.Line, combo, Missing, test})
			}
		}
	}
	return findings
}

// rowFor finds the row covering a combination. spec-lint has already
// established disjointness, so the first match is the only match.
func rowFor(tb specparser.Table, domains []specparser.Domain, combo Combination) ([]string, bool) {
	for _, row := range tb.Rows {
		if specparser.RowMatches(row, domains, combo) {
			return row, true
		}
	}
	return nil, false
}

// namedTestExists accepts a cell naming one or more tests, separated by
// commas -- one row is often enforced by several.
func namedTestExists(cell string, names map[string]bool) bool {
	for _, part := range strings.Split(cell, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !names[part] {
			return false
		}
	}
	return true
}
