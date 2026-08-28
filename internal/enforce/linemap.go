package enforce

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Which tests execute which line, so an adversary costs a couple of tests
// instead of the whole suite.
//
// Without this, one adversary costs one full verifier run: 41 seconds on a
// real Shipd task, so a hundred adversaries is over an hour. Three things
// remove almost all of that, and none of them is extra work:
//
//  1. The map is free. ray must already run the suite once to confirm the
//     unmodified solution passes -- without that baseline no adversary
//     result means anything. Turning coverage on during THAT run costs
//     nothing more.
//
//  2. An adversary that does not change behaviour is skipped before any
//     test runs. The literature calls this state infection: a mutant
//     cannot be killed unless the mutated expression's value differs.
//     That is the probe check, and it is cheap.
//
//  3. An adversary on a line NO test executes cannot be killed by
//     anything. It needs no test run at all -- it is a finding by
//     construction. Measured mutation coverage in real projects is often
//     below 50%, so this alone removes about half of them.
//
// Only what remains runs tests, and only the tests that touch that line.

// LineMap answers "which tests execute this line of this file?".
type LineMap map[string]map[int][]string

// LineMapScript resolves the collector relative to this file, so it works
// from any working directory.
func LineMapScript() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("enforce: could not resolve the line-map collector's path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..",
		"third_party", "coverage", "line_tests.py"), nil
}

// BuildLineMap runs the suite once with coverage recording which test was
// active for each executed line.
func BuildLineMap(pythonPath, root, tests, source string) (LineMap, error) {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	script, err := LineMapScript()
	if err != nil {
		return nil, err
	}
	out, err := exec.Command(pythonPath, script,
		"--root", root, "--tests", tests, "--source", source).Output()
	if err != nil {
		return nil, fmt.Errorf("line map collection failed (is pytest-cov installed?): %w", err)
	}

	var raw map[string]map[string][]string
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing the line map: %w", err)
	}

	m := LineMap{}
	for file, lines := range raw {
		converted := map[int][]string{}
		for lineno, tests := range lines {
			var n int
			if _, err := fmt.Sscanf(lineno, "%d", &n); err == nil {
				converted[n] = tests
			}
		}
		m[filepath.Base(file)] = converted
	}
	return m, nil
}

// TestsForLine reports the tests that execute a line, and whether the map
// has anything for that file at all.
//
// The second result matters: "no test executes this line" and "this file
// was never measured" look identical otherwise, and treating an unmeasured
// file as untested would report every adversary in it as a finding.
func (m LineMap) TestsForLine(file string, line int) ([]string, bool) {
	lines, measured := m[filepath.Base(file)]
	if !measured {
		return nil, false
	}
	return lines[line], true
}

// Covered reports whether any test executes the line. Only meaningful when
// the file was measured.
func (m LineMap) Covered(file string, line int) bool {
	tests, measured := m.TestsForLine(file, line)
	return measured && len(tests) > 0
}

// SelectCommand builds a command running only the given tests, from the
// task's own single-test template. Falls back to the empty string when the
// task provides no template, so the caller runs the full verifier instead.
func SelectCommand(oneTest string, tests []string) string {
	if oneTest == "" || len(tests) == 0 {
		return ""
	}
	// pytest's -k takes an or-expression, which is how a task's template
	// already selects one test by name.
	return strings.ReplaceAll(oneTest, "{test}", strings.Join(tests, " or "))
}
