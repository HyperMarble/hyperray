// Package intent checks the frozen spec against the third evidence
// source: the instruction the solver actually receives.
//
// spec.md is cross-checked against the code (sufficiency) and against the
// tests (coverage, enforce). The instruction was written separately from
// both, so it is independent evidence about what the task means -- the
// same three-source method OpenAI's own SWE-bench audit uses: problem
// statement, tests, and reference solution.
//
// The 2026-08-25 design said ray never parses instruction.md. That is
// revised deliberately: not parsing it left the spec checked against two
// sources out of three, and the direction it misses is the one that makes
// a task unfair rather than lax.
//
// A row required by the spec but absent from the instruction means the
// solver is graded on something it was never told. That is the unfair-fail
// direction, and it is a defect in the task even though no false positive
// follows from it.
//
// WHAT IS MATCHED, AND WHY ONLY THIS. Matching a row's prose against the
// instruction by shared identifiers was measured across 113 real tasks and
// flagged 95% of them, offering `bool`, `json` and `data` as
// "requirements". Useless. So only the QUOTED text of a row is matched --
// the string in `raise X containing "..."`. That text is a deliberate,
// specific choice by whoever wrote the spec, and it either appears in the
// instruction or it does not. No judgement, no fuzziness, no noise.
//
// Rows without quoted text are not checked and are reported as such, never
// as satisfied.
package intent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/HyperMarble/ray/internal/specparser"
)

// Status is what is known about one row's presence in the instruction.
type Status int

const (
	// Stated: the row's quoted requirement appears in the instruction.
	Stated Status = iota
	// Unstated: it does not. The solver cannot know about it.
	Unstated
	// Unquotable: the row states no quoted text, so nothing can be matched
	// without the judgement this package refuses to make.
	Unquotable
)

// Finding is one row and what is known about it.
type Finding struct {
	Section  string `json:"section"`
	Behavior string `json:"behavior"`
	Quoted   string `json:"quoted,omitempty"`
	Status   Status `json:"status"`
}

func (f Finding) String() string {
	switch f.Status {
	case Unstated:
		return fmt.Sprintf("%s: requires %q, which instruction.md never mentions", f.Section, f.Quoted)
	case Unquotable:
		return fmt.Sprintf("%s: %q states no quoted requirement, so it cannot be matched against instruction.md",
			f.Section, shorten(f.Behavior))
	}
	return fmt.Sprintf("%s: %q is stated", f.Section, f.Quoted)
}

// quoted pulls the exact text a row requires a failure to carry.
var quoted = regexp.MustCompile(`containing\s+"([^"]+)"`)

// Check reports, for every row of every table, whether its quoted
// requirement appears in the instruction.
func Check(tables []specparser.Table, instruction string) []Finding {
	haystack := normalise(instruction)

	var findings []Finding
	seen := map[string]bool{}
	for _, tb := range tables {
		n := tb.ParamColumns()
		for _, row := range tb.Rows {
			if n >= len(row) {
				continue
			}
			behavior := strings.TrimSpace(row[n])
			if behavior == "" {
				continue
			}
			m := quoted.FindStringSubmatch(behavior)
			if m == nil {
				findings = append(findings, Finding{tb.Section, behavior, "", Unquotable})
				continue
			}
			text := m[1]
			// One row per distinct requirement: a compound row expands to
			// several combinations but states the requirement once, and
			// reporting it repeatedly buries the others.
			key := tb.Section + "\x00" + text
			if seen[key] {
				continue
			}
			seen[key] = true

			status := Unstated
			if strings.Contains(haystack, normalise(text)) {
				status = Stated
			}
			findings = append(findings, Finding{tb.Section, behavior, text, status})
		}
	}
	return findings
}

// Unstated returns only the rows the instruction never mentions.
func Unstated_(findings []Finding) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Status == Unstated {
			out = append(out, f)
		}
	}
	return out
}

var spaces = regexp.MustCompile(`\s+`)

// normalise lowercases and collapses whitespace, so a requirement written
// across two lines in the instruction still matches one written on a
// single line in the spec. Nothing else is normalised: the point is an
// exact match on a deliberately chosen string.
func normalise(s string) string {
	return spaces.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), " ")
}

func shorten(s string) string {
	if len(s) > 70 {
		return s[:67] + "..."
	}
	return s
}
