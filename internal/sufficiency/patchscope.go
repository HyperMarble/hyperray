package sufficiency

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Scoping the sufficiency check to what the task actually changed.
//
// A repo-modification task adds a feature to somebody else's library. The
// solution file is mostly code the task never touches: on a real deep-swe
// task, `stencil.py` is 1000+ lines of numba, of which the task changes
// about 200. Reading the whole file made sufficiency report 19 outcomes
// "outside the frozen contract" -- every one of them a pre-existing numba
// error unrelated to the task, and none of them something spec.md should
// ever have described.
//
// The frozen spec describes the task. So sufficiency must ask its question
// only of the lines the task's own patch introduced.

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// ChangedLines parses a unified diff and reports, per file, which lines of
// the POST-patch file the patch introduced.
//
// Files are keyed by base name. A patch names paths relative to the repo
// root ("b/numba/stencils/stencil.py") while a task points at the file
// wherever it happens to sit, so a full-path match would almost never
// succeed. Base names collide in principle; in a single task's patch that
// has not been observed, and a collision widens the scope rather than
// narrowing it -- it can only cause extra outcomes to be considered, never
// cause a real one to be dropped.
func ChangedLines(patch string) map[string]map[int]bool {
	changed := map[string]map[int]bool{}
	var file string
	line := 0

	for _, raw := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(raw, "+++ "):
			p := strings.TrimSpace(strings.TrimPrefix(raw, "+++ "))
			p = strings.TrimPrefix(p, "b/")
			if p == "/dev/null" {
				file = ""
				continue
			}
			file = filepath.Base(p)
			if changed[file] == nil {
				changed[file] = map[int]bool{}
			}
		case hunkHeader.MatchString(raw):
			m := hunkHeader.FindStringSubmatch(raw)
			line, _ = strconv.Atoi(m[1])
		case file == "":
			continue
		case strings.HasPrefix(raw, "+"):
			changed[file][line] = true
			line++
		case strings.HasPrefix(raw, "-"):
			// Removed lines occupy no line in the post-patch file.
		case strings.HasPrefix(raw, " "):
			line++
		}
	}
	return changed
}

// ScopeToPatch keeps only the outcomes the patch introduced.
//
// A patch with nothing for this file leaves the outcomes untouched: a
// greenfield task, where the solution file IS the task, must still be
// judged in full. Narrowing on an absent patch would silently stop
// checking anything.
func ScopeToPatch(outcomes []Outcome, sourcePath, patch string) []Outcome {
	changed := ChangedLines(patch)
	lines, ok := changed[filepath.Base(sourcePath)]
	if !ok || len(lines) == 0 {
		return outcomes
	}
	kept := make([]Outcome, 0, len(outcomes))
	for _, o := range outcomes {
		if lines[o.Line] {
			kept = append(kept, o)
		}
	}
	return kept
}

// AddedSource returns the lines a patch adds to one file, joined as text.
//
// This is what makes a patch-shaped task readable without its environment.
// The statements a spec row names are all in the patch -- it IS the task's
// new code -- so hyperray can find them from a 47KB file on disk instead of
// reaching into a 4.6GB container. Line numbers are not preserved and are
// not needed: a violation is located by matching statement TEXT, not by
// position.
func AddedSource(patch, base string) string {
	var out []string
	var file string
	for _, raw := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(raw, "+++ "):
			p := strings.TrimSpace(strings.TrimPrefix(raw, "+++ "))
			file = filepath.Base(strings.TrimPrefix(p, "b/"))
		case file != base:
			continue
		case strings.HasPrefix(raw, "+"):
			out = append(out, raw[1:])
		}
	}
	return strings.Join(out, "\n")
}
