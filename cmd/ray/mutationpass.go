package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HyperMarble/ray/internal/mutate"
)

// runMutationPass asks the question no other pass asks: would the task's
// own tests notice if the solution stopped meeting a requirement?
//
// It breaks the solution one operator or constant at a time and runs the
// task's real test suite against each. A mutant the suite fails to
// notice is a requirement the suite does not verify -- the exact false
// positive where an agent meets A and B, skips C, and still passes.
//
// It needs three things the other passes do not: a real solution source
// to mutate, a runnable test command, and a working directory to run it
// in. Missing any of them means the pass is reported as skipped, never
// as clean.
// runPython is the interpreter the equivalence check executes the
// solution under. It must be able to import the solution's dependencies,
// so it is the one that runs the tests -- NOT genPython, which only needs
// tree_sitter to parse. Using the parser's interpreter here made every
// survivor in a real sktime run come back "unchecked", because
// _rowwise.py imports skpro and that interpreter has no skpro.
func runMutationPass(solutionPath, lang, genPython, runPython string, testCmd []string,
	workDir string, timeout time.Duration, harvested []any) passResult {

	start := time.Now()
	p := passResult{name: "mutation"}

	if solutionPath == "" {
		p.state, p.summary, p.dur = passAdvisory, "no --solution given", time.Since(start)
		return p
	}
	if len(testCmd) == 0 {
		p.state, p.summary, p.dur = passAdvisory, "no --test-cmd given", time.Since(start)
		return p
	}
	if _, err := os.Stat(solutionPath); err != nil {
		p.state, p.summary, p.dur = passAdvisory, "solution not readable", time.Since(start)
		return p
	}
	if workDir == "" {
		workDir = filepath.Dir(solutionPath)
	}

	mutants, err := mutate.Generate(genPython, solutionPath, lang)
	if err != nil {
		p.state, p.summary, p.dur = passAdvisory, shorten(err.Error()), time.Since(start)
		return p
	}
	if len(mutants) == 0 {
		p.state, p.dur = passAdvisory, time.Since(start)
		p.summary = "no mutable operators or constants in the solution"
		return p
	}

	// The unmodified solution must pass before any adversary's result means
	// anything. Measured on a real Pluto task: the discovered command needed
	// WORKSPACE/TESTS_DIR/LOG_ROOT and died on its first line without them,
	// so every adversary "failed" it, every failure counted as caught, and
	// this pass printed "93 mutant(s), all caught by the tests" -- a clean
	// bill of health from tests that never ran, printed next to three proven
	// false positives.
	//
	// The killed==0 guard below catches the mirror image of this and misses
	// this case entirely: a command that always fails kills EVERY adversary.
	if base, err := os.ReadFile(solutionPath); err == nil {
		ok, err := mutate.RunTests(solutionPath, string(base), testCmd, workDir, timeout)
		if err != nil {
			p.state, p.summary, p.dur = passAdvisory, shorten(err.Error()), time.Since(start)
			return p
		}
		if !ok {
			p.state, p.dur = passAdvisory, time.Since(start)
			p.summary = "the unmodified solution does not pass the discovered test command — no adversary result is meaningful"
			return p
		}
	}

	killed := 0
	var survivors []mutate.Mutant
	for _, m := range mutants {
		passed, err := mutate.RunTests(solutionPath, m.Source, testCmd, workDir, timeout)
		if err != nil {
			// A restore failure is reported loudly: leaving a task's
			// solution mutated on disk is worse than any missed finding.
			p.state, p.dur = passAdvisory, time.Since(start)
			p.summary = "aborted"
			p.findings = append(p.findings, shorten(err.Error()))
			return p
		}
		if passed {
			survivors = append(survivors, m)
		} else {
			killed++
		}
	}

	p.dur = time.Since(start)

	// If nothing was killed the suite never really ran, so a survivor
	// would prove nothing. Say that rather than report every mutant as a
	// gap.
	if killed == 0 {
		p.state = passAdvisory
		p.summary = fmt.Sprintf("%d mutant(s), none killed — the test command may not be running", len(mutants))
		return p
	}

	if len(survivors) == 0 {
		p.state = passAdvisory
		p.summary = fmt.Sprintf("%d mutant(s), all caught by the tests", len(mutants))
		return p
	}

	// Surviving the tests is not yet a finding. A mutant that cannot
	// behave differently on any input is equivalent -- no test could have
	// caught it, and reporting it would send the agent chasing nothing.
	// Only survivors shown to genuinely differ are reported, each with
	// the input that proves it.
	originalSrc, err := os.ReadFile(solutionPath)
	if err != nil {
		p.state, p.summary, p.dur = passAdvisory, "could not re-read solution", time.Since(start)
		return p
	}
	// Call the function each mutant actually lives in. Calling one fixed
	// function proves nothing about a mutant somewhere else -- a real
	// pluto task had 74 of 75 mutants outside the function being called.
	fnByLine, err := mutate.FunctionsByLine(genPython, solutionPath)
	inputs := append(mutate.BoundaryInputs(mutants), harvested...)

	var gaps []mutate.Mutant
	witnesses := map[int]string{}
	equivalent, unchecked := 0, 0
	for _, m := range survivors {
		fnName := fnByLine[m.Line]
		// A method needs an instance to call, which ray does not
		// construct. Reporting that as unchecked is honest; comparing
		// some other function instead would not be.
		if err != nil || len(inputs) == 0 || fnName == "" || strings.Contains(fnName, ".") {
			unchecked++
			continue
		}
		differs, witness, derr := mutate.DiffersFromOriginal(
			runPython, string(originalSrc), m.Source, fnName, inputs)
		if derr != nil {
			unchecked++
			continue
		}
		if !differs {
			equivalent++
			continue
		}
		gaps = append(gaps, m)
		witnesses[m.ID] = witness
	}

	p.dur = time.Since(start)
	if len(gaps) == 0 {
		p.state = passAdvisory
		p.summary = fmt.Sprintf("%d mutant(s): %d caught by tests, %d equivalent, %d unchecked",
			len(mutants), killed, equivalent, unchecked)
		return p
	}

	p.state = passAdvisory
	p.summary = fmt.Sprintf("%d requirement(s) the tests do not verify (of %d mutants; %d equivalent)",
		len(gaps), len(mutants), equivalent)
	for i, m := range gaps {
		if i == 8 {
			p.findings = append(p.findings, fmt.Sprintf("… and %d more", len(gaps)-8))
			break
		}
		p.findings = append(p.findings, fmt.Sprintf(
			"L%d: %s %q → %q behaves differently at %s, yet tests still pass",
			m.Line, m.Operator, m.Original, m.Mutated, witnesses[m.ID]))
	}
	return p
}
