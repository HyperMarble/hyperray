package enforce

import (
	"regexp"
	"strings"

	"github.com/HyperMarble/hyperray/internal/specparser"
	"github.com/HyperMarble/hyperray/internal/sufficiency"
)

// Deriving a violation from the frozen spec and the real source, rather
// than having an author hand-write one per row.
//
// A row that says `raise <Type> containing "<text>"` names exactly one
// statement in the solution: the one whose source text carries that text.
// Neutralising that statement is, by construction, a violation of that
// row. Nothing is guessed -- the row supplies the text, tree-sitter
// supplies the statement, and the match is exact substring containment.
//
// This is what makes obligation B run on an arbitrary task. Hand-authored
// violations still win when present, because an author can express
// violations this cannot derive (a wrong value rather than a missing
// guard), but their absence no longer stops hyperray from reporting.

// neutraliser is the text that replaces a removed terminating statement.
//
// Removing the statement outright leaves a syntactically empty block in
// most languages, which is legal -- but not in Python, whose block must
// contain something. That is real language semantics, not a special case
// per project, so it is declared here and nowhere else.
var neutraliser = map[string]string{
	"python": "pass",
	"rust":   "",
	"cpp":    "",
	"go":     "",
}

// Synthesize derives one violation per spec row whose Required behavior
// names a failure, by locating the statement in the solution that carries
// the required text.
func Synthesize(tables []specparser.Table, language string, outcomes []sufficiency.Outcome) []ScopedViolation {
	with, ok := neutraliser[language]
	if !ok {
		return nil
	}

	var out []ScopedViolation
	for _, tb := range tables {
		domains, _, err := specparser.ParseParams(tb.Params)
		if err != nil {
			continue
		}
		n := tb.ParamColumns()
		for _, row := range tb.Rows {
			if n >= len(row) {
				continue
			}
			want, ok := ParseExpectation(strings.TrimSpace(row[n]))
			if !ok || want.Contains == "" {
				// Only a row naming the text its failure must carry can be
				// matched to a statement unambiguously. A row requiring a
				// value has no single statement to remove, and guessing one
				// would be exactly the invention this avoids.
				continue
			}
			stmt := statementCarrying(outcomes, want.Contains)
			if stmt == "" {
				continue
			}
			out = append(out, ScopedViolation{
				Section: tb.Section,
				When:    rowScope(row, domains),
				Violation: Violation{
					Cut:  stmt,
					With: with,
				},
			})
		}
	}
	return out
}

// statementCarrying finds the single raise/throw whose source text carries
// the required text. Ambiguity is refused rather than resolved: if two
// statements carry it, neither is provably the row's, and reporting on a
// guess is how a verifier starts lying.
func statementCarrying(outcomes []sufficiency.Outcome, text string) string {
	var found string
	for _, o := range outcomes {
		if o.Kind != "raise" {
			continue
		}
		if !strings.Contains(normalise(o.SourceText), normalise(text)) {
			continue
		}
		if found != "" {
			return ""
		}
		found = o.SourceText
	}
	return found
}

// normalise collapses whitespace so a message split across source lines
// still matches the row's single-line text.
var spaces = regexp.MustCompile(`\s+`)

func normalise(s string) string {
	return spaces.ReplaceAllString(strings.TrimSpace(s), " ")
}

// rowScope turns a row's cells into the parameter values that identify it,
// skipping wildcard columns, which place no restriction.
func rowScope(row []string, domains []specparser.Domain) map[string]string {
	scope := map[string]string{}
	for c, d := range domains {
		if c >= len(row) {
			break
		}
		vals := specparser.CellValues(row[c], d)
		// A cell admitting every declared value constrains nothing, and a
		// compound cell names several -- neither identifies the row, so
		// only a cell naming exactly one value contributes.
		if len(vals) == 1 && len(d.Values) > 1 {
			scope[d.Name] = vals[0]
		}
	}
	return scope
}
