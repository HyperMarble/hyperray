package specparser

import "strings"

// EnforcedByHeader names the optional final column in which a row
// declares the test that enforces it.
//
// Layer 2's question -- "is there a test for this requirement?" -- cannot
// be answered by matching the row's text against test code. Measured on
// 113 real tasks, identifier overlap between a prompt and its tests
// flags 95% of them, offering `bool`, `json` and `data` as requirements.
// Whether a test enforces a requirement is a question about meaning, so
// the author states the answer and ray verifies it, rather than ray
// guessing.
const EnforcedByHeader = "Enforced by"

// EnforcedByIndex returns the index of the "Enforced by" column, or -1
// when the table does not declare one.
func (t Table) EnforcedByIndex() int {
	for i, col := range t.Columns {
		if strings.EqualFold(strings.TrimSpace(col), EnforcedByHeader) {
			return i
		}
	}
	return -1
}

// ParamColumns reports how many leading columns hold parameter values. Strict
// tables contain several metadata columns, so the boundary is the first known
// semantic metadata header rather than a fixed count from the end.
func (t Table) ParamColumns() int {
	metadata := map[string]bool{
		"id": true, "operation": true, "reachability": true,
		"required behavior": true, "required outcomes": true, "forbidden outcomes": true,
		"effects": true, "invariants": true, "input witnesses": true,
		"enforced by": true, "instruction source": true, "constraint reason": true,
	}
	for index, column := range t.Columns {
		if metadata[strings.ToLower(strings.TrimSpace(column))] {
			return index
		}
	}
	return len(t.Columns)
}

// RequiredBehavior returns a row's required-outcome cell, located by header
// name. Position cannot be assumed: legacy tables put the behavior right
// after the parameters, but strict tables put ID/Operation/Reachability
// first, and reading by position there returns the row's ID -- which is how
// a generated fix once asserted `== no-route`.
func (t Table) RequiredBehavior(row []string) string {
	for i, col := range t.Columns {
		name := strings.ToLower(strings.TrimSpace(col))
		if name == "required outcomes" || name == "required behavior" {
			if i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
	}
	if n := t.ParamColumns(); n < len(row) {
		return strings.TrimSpace(row[n])
	}
	return ""
}

// EnforcedBy returns the test named by a row, or "" when the row names
// none. An empty cell is a row nothing is declared to enforce -- which
// is a finding, not an error.
func (t Table) EnforcedBy(row []string) string {
	i := t.EnforcedByIndex()
	if i < 0 || i >= len(row) {
		return ""
	}
	cell := strings.TrimSpace(strings.ReplaceAll(row[i], "`", ""))
	// The strict schema writes the no-test case as the keyword `none`; treating
	// it as a test named "none" made isolation judge a phantom test.
	if strings.EqualFold(cell, "none") {
		return ""
	}
	return cell
}

// CellValues expands one already-validated table cell into the set of declared values it
// admits: a wildcard covers the whole domain, a quote-aware ` / `-separated list
// covers each of its members, and anything else covers itself. A cell
// naming a value the domain never declared contributes nothing, which
// spec-lint reports separately as an undeclared value.
func CellValues(cell string, domain Domain) []string {
	cell = strings.TrimSpace(cell)
	if isWildcardCell(cell) {
		return domain.Values
	}
	values, _, err := ParseValueList(cell)
	if err != nil {
		return nil
	}
	var out []string
	for _, tok := range values {
		for _, v := range domain.Values {
			if v == tok {
				out = append(out, tok)
				break
			}
		}
	}
	return out
}

// RowMatches reports whether a row admits a combination -- every column's
// cell must allow that combination's value for the column.
func RowMatches(row []string, domains []Domain, combo map[string]string) bool {
	for c, d := range domains {
		if c >= len(row) {
			return false
		}
		want, ok := combo[d.Name]
		if !ok {
			return false
		}
		found := false
		for _, v := range CellValues(row[c], d) {
			if v == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// isWildcardCell reports the two reserved keywords: `any` (every declared
// value applies) and an em dash (the column does not apply to this row).
// Both mean the row places no restriction on that column.
func isWildcardCell(cell string) bool {
	c := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(cell, "`", "")))
	return c == "any" || c == "—" || c == "-" || c == "--" || c == ""
}
