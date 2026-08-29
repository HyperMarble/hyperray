// Package sufficiency implements the one requirement spec-lint cannot:
// does spec.md's set of tables actually account for the real source's
// actual outcomes, not just agree with itself internally. spec-lint only
// checks that declared rows don't conflict; it has no way to know a real
// outcome in the source was never written into any row at all.
package sufficiency

import (
	"regexp"
	"strings"

	"github.com/HyperMarble/hyperray/internal/specparser"
)

// Gap is a real outcome spec.md never mentions.
type Gap struct {
	Line       int
	Kind       string // "raise" or "return"
	SourceText string
}

var quotedRe = regexp.MustCompile(`"([^"]+)"`)

// CheckRaiseCoverage cross-references every real raise/throw/panic
// against spec.md's Required-behavior text, using the same
// `containing "X"` convention the spec skill itself teaches: a raise is
// covered when some phrase spec.md quotes appears verbatim in the raise's
// own source text. Matching the whole statement text needs no derived
// exception-type or message field -- confirmed against the real sktime
// source to find the identical gaps those hand-parsed fields did.
//
// Return outcomes are intentionally not checked here. A return's source
// text is usually a computed expression (`return n + 1`), not quotable
// prose, so every return would report as a gap and the signal would be
// noise. That is a real, honest limit of text matching as a technique --
// establishing that a return's value satisfies a row is a job for the
// oracle (Layer 3), not for string comparison, and no weaker per-shape
// heuristic should be invented to paper over it.
func CheckRaiseCoverage(tables []specparser.Table, outcomes []Outcome) []Gap {
	var sb strings.Builder
	for _, tb := range tables {
		for _, row := range tb.Rows {
			for _, cell := range row {
				sb.WriteString(cell)
				sb.WriteByte('\n')
			}
		}
	}
	allText := sb.String()

	var phrases []string
	for _, m := range quotedRe.FindAllStringSubmatch(allText, -1) {
		phrases = append(phrases, m[1])
	}

	var gaps []Gap
	for _, o := range outcomes {
		if o.Kind != "raise" {
			continue
		}
		covered := false
		for _, phrase := range phrases {
			if strings.Contains(o.SourceText, phrase) {
				covered = true
				break
			}
		}
		if !covered {
			gaps = append(gaps, Gap{Line: o.Line, Kind: o.Kind, SourceText: o.SourceText})
		}
	}
	return gaps
}
