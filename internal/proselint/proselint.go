// Package proselint checks the task's problem statement the way the
// submission platform and its reviewers do, before anything is uploaded:
// plain ASCII only (the platform rejects the first non-ASCII byte), a word
// budget the style guide asks for, and -- the deep check -- promise-word
// coverage: a sentence containing a loaded word like "equals" or
// "preserves" commits the task to behavior, so a statement line carrying
// such a word with no spec row anchored to it is a promise nothing models.
// The linter never interprets the English; it only demands that every
// promise-bearing line is accounted for in the spec.
package proselint

import (
	"fmt"
	"strings"
)

// promiseWords are the loaded words: each one commits the statement to
// checkable behavior with more parts than the word shows. The list is a
// closed dictionary on purpose -- deterministic output, same input, same
// findings, every run.
var promiseWords = []string{
	"equals", "equal",
	"preserves", "preserve", "preserving",
	"agrees", "agree", "agreeing",
	"keeps", "keep", "keeping",
	"matches", "match", "matching",
	"mirrors", "mirror",
	"identical",
	"unchanged",
	"containing",
	"same",
	"every",
	"regardless",
	"mapping", "mappings",
	"sequence", "sequences",
	"iterable", "iterables",
	"collection", "collections",
}

// PromiseLine is one statement line carrying promise words, with the count
// of spec rows anchored to it.
type PromiseLine struct {
	Line  int
	Words []string
	Rows  int
}

// NonASCII locates the first non-ASCII byte, mirroring the platform's own
// rejection. Position is zero-based byte offset; found reports existence.
func NonASCII(text string) (position int, char rune, found bool) {
	for index, r := range text {
		if r > 127 {
			return index, r, true
		}
	}
	return 0, 0, false
}

// CountWords counts whitespace-separated words.
func CountWords(text string) int {
	return len(strings.Fields(text))
}

// PromiseLines lists every statement line containing at least one promise
// word, annotated with the number of spec rows anchored to that line via
// rowsPerLine (one-based line number -> anchored row count).
func PromiseLines(text string, rowsPerLine map[int]int) []PromiseLine {
	var found []PromiseLine
	for number, line := range strings.Split(text, "\n") {
		words := promiseWordsIn(line)
		if len(words) == 0 {
			continue
		}
		lineNumber := number + 1
		found = append(found, PromiseLine{
			Line:  lineNumber,
			Words: words,
			Rows:  rowsPerLine[lineNumber],
		})
	}
	return found
}

// Uncovered filters PromiseLines down to the failures: lines that promise
// behavior and have no spec row anchored to them at all.
func Uncovered(lines []PromiseLine) []PromiseLine {
	var uncovered []PromiseLine
	for _, line := range lines {
		if line.Rows == 0 {
			uncovered = append(uncovered, line)
		}
	}
	return uncovered
}

// Describe renders one promise line for terminal output.
func Describe(line PromiseLine) string {
	return fmt.Sprintf("line %d promises %s -- %d spec row(s) anchored", line.Line, strings.Join(line.Words, ", "), line.Rows)
}

func promiseWordsIn(line string) []string {
	lowered := strings.ToLower(line)
	tokens := map[string]bool{}
	for _, field := range strings.Fields(lowered) {
		tokens[strings.Trim(field, ".,;:()`\"'-*")] = true
	}
	var found []string
	for _, word := range promiseWords {
		if tokens[word] {
			found = append(found, word)
		}
	}
	return found
}
