// Tests for the prose gate: non-ASCII detection mirrors the platform's
// byte-level rejection, and promise-word coverage demands a spec row behind
// every line that commits the statement to behavior.
package tests

import (
	"testing"

	"github.com/HyperMarble/hyperray/internal/proselint"
)

func TestProseLint_FindsFirstNonASCIIByte(t *testing.T) {
	position, char, found := proselint.NonASCII("plain then an em dash — here")
	if !found {
		t.Fatal("expected the em dash to be found")
	}
	if char != '—' {
		t.Fatalf("expected the em dash, got %q", char)
	}
	if position != 22 {
		t.Fatalf("expected byte position 22, got %d", position)
	}
}

func TestProseLint_CleanASCIIPasses(t *testing.T) {
	if _, _, found := proselint.NonASCII("all seven-bit text -- with plain dashes"); found {
		t.Fatal("expected no finding on pure ASCII")
	}
}

func TestProseLint_PromiseLinesCarryAnchorsAndWords(t *testing.T) {
	text := "This line promises nothing.\nThe result equals the composed value.\nAnd every route keeps its flag."
	lines := proselint.PromiseLines(text, map[int]int{2: 3})
	if len(lines) != 2 {
		t.Fatalf("expected 2 promise lines, got %d: %+v", len(lines), lines)
	}
	if lines[0].Line != 2 || lines[0].Rows != 3 {
		t.Fatalf("line 2 should carry 3 anchored rows, got %+v", lines[0])
	}
	if lines[1].Line != 3 || lines[1].Rows != 0 {
		t.Fatalf("line 3 should carry 0 anchored rows, got %+v", lines[1])
	}
}

func TestProseLint_UncoveredKeepsOnlyZeroAnchorLines(t *testing.T) {
	text := "The order matches the input.\nThe cutoff stays unchanged."
	lines := proselint.PromiseLines(text, map[int]int{1: 1})
	uncovered := proselint.Uncovered(lines)
	if len(uncovered) != 1 || uncovered[0].Line != 2 {
		t.Fatalf("expected only line 2 uncovered, got %+v", uncovered)
	}
}

func TestProseLint_PromiseWordsSurvivePunctuationAndCase(t *testing.T) {
	text := "Views agree; labels stay (identical), `containing` the fragment."
	lines := proselint.PromiseLines(text, nil)
	if len(lines) != 1 {
		t.Fatalf("expected one promise line, got %+v", lines)
	}
	if len(lines[0].Words) != 3 {
		t.Fatalf("expected agree, identical, containing to be found, got %v", lines[0].Words)
	}
}

func TestProseLint_TypeFamilyWordsAreLoaded(t *testing.T) {
	text := "fh_params accepts mappings and lists.\nInput may be any sequence type."
	lines := proselint.PromiseLines(text, nil)
	if len(lines) != 2 {
		t.Fatalf("expected both type-family lines flagged, got %+v", lines)
	}
}

func TestProseLint_CountWords(t *testing.T) {
	if got := proselint.CountWords("one two  three\nfour"); got != 4 {
		t.Fatalf("expected 4 words, got %d", got)
	}
}
