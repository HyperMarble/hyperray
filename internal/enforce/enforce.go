// Package enforce discharges obligation B -- grader soundness -- for one
// expanded specification combination at a time.
//
// The question is not whether the solution is correct. It is whether a
// solution that VIOLATES a frozen requirement can still pass the task's
// tests. So for each obligation hyperray constructs a targeted violation,
// confirms with a witness that the modified solution really does violate
// it, and then runs the task's own verifier:
//
//	verifier still passes            -> proven false positive
//	verifier fails via declared test -> enforced
//	verifier fails some other way    -> misdeclared; the suite rejects it,
//	                                    but not through the test the spec
//	                                    names, so traceability is broken
//	violation not demonstrable       -> inconclusive, never "enforced"
//
// Isolation matters and is why the declared test is checked by name
// rather than trusting a red suite. Measured on a real task: deleting the
// wrap-around guard also emptied the range, so a DIFFERENT rule raised
// the same exception type, the declared test went green, and the
// obligation was silently unenforced. A killed violation is evidence only
// when what died is the thing that was supposed to die.
package enforce

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Violation is how to break exactly one obligation: replace Cut with
// With in File, then run Witness to prove the behaviour actually changed.
type Violation struct {
	File    string `json:"file"`
	Cut     string `json:"cut"`
	With    string `json:"with"`
	Witness string `json:"witness"`
}

// Obligation is one expanded combination together with the test the
// frozen spec declares enforces it and the edit that violates it.
type Obligation struct {
	Section string            `json:"section"`
	Combo   map[string]string `json:"when"`
	Test    string            `json:"test"`
	// Behavior is the row's Required-behavior cell, verbatim. Obligation A
	// is judged against it; obligation B never reads it.
	Behavior  string    `json:"behavior"`
	Violation Violation `json:"violation"`
}

// Task describes how to edit and verify. Nothing here knows a language or
// a runner: the only operations are "replace text in a file" and "run a
// command", so Python, Rust, C++, Go, pytest, cargo and a bare shell
// script are all the same. The file need not be on this filesystem --
// real bench tasks keep their source inside a container, because the repo
// needs its own toolchain to run at all.
type Task struct {
	SourceRoot string `json:"source_root"`
	// SolutionFile is the file synthesized violations edit, relative to
	// SourceRoot. Authored violations name their own file.
	SolutionFile string `json:"solution_file"`
	TestCwd      string `json:"test_cwd"`
	// TestCommand runs the task's own verifier.
	TestCommand string `json:"test_command"`
	// OneTest runs a single named test; {test} is substituted. Optional --
	// without it, isolation is judged from the full run's output.
	OneTest string `json:"one_test"`
	// PassFile holds the verdict when exit status is not the pass signal.
	// Harbor-style test.sh ends in `exit 0` unconditionally and writes
	// /logs/verifier/reward.txt, so trusting the exit code marks every
	// obligation enforced against that bench, silently, forever.
	PassFile  string `json:"pass_file"`
	PassValue string `json:"pass_value"`
	// ReadFile/WriteFile reach source that is not local, e.g.
	//   "docker exec NAME cat {file}" / "docker exec -i NAME tee {file}"
	ReadFile  string `json:"read_file"`
	WriteFile string `json:"write_file"`
	// TestList prints the task's test names, one per line. Needed when the
	// tests are not on this filesystem: coverage's declarative half scans
	// a local directory, and without this a containerised task can never
	// be certified, because the layer reports itself skipped forever.
	TestList string `json:"test_list"`
	// Probes are inputs the real solution and an adversary are both run
	// on, so a difference in behaviour can be demonstrated. Without them
	// an adversary is only a guess: nothing shows it actually deviates.
	Probes []Probe `json:"probes"`
	// ProbeShape declares the shape of an input, as a Hypothesis strategy.
	// hyperray generates the concrete probes from it, so which inputs get tried
	// is not a person's guess.
	ProbeShape string `json:"probe_shape"`
	// ProbeCommand runs the program on one generated input, substituted at
	// {input}. The task keeps ownership of how an input reaches it.
	ProbeCommand string `json:"probe_command"`
	ProbeCount   int    `json:"probe_count"`
	// CoverageTests and CoverageSource let hyperray record which tests execute
	// which line during the baseline run it has to do anyway. Absent them,
	// every adversary costs a full verifier run.
	// TestsDir is where the task's tests live, filled in by discovery so a
	// task need not declare it.
	TestsDir       string `json:"tests_dir"`
	CoverageTests  string `json:"coverage_tests"`
	CoverageSource string `json:"coverage_source"`
	// ProbeBatchCommand, when set, observes every probe in one process:
	// {probes} is replaced with the space-joined script paths and the
	// command's output carries ===HYPERRAY_PROBE <basename>=== sections. Every
	// probe still runs completely; only the per-probe process start and
	// import cost is removed.
	ProbeBatchCommand string `json:"probe_batch_command"`
	// AfterWrite clears anything cached from the old source. Python keeps
	// bytecode beside the source; a stale .pyc means the edit never takes
	// effect and every obligation looks unviolated.
	AfterWrite string `json:"after_write"`
	Timeout    time.Duration
}

type Verdict int

const (
	// Inconclusive: the violation could not be demonstrated, so nothing
	// about enforcement is known. Never silently promoted to Enforced.
	Inconclusive Verdict = iota
	// Enforced: the declared test failed on the violating solution.
	Enforced
	// FalsePositive: the violating solution passed the whole verifier.
	FalsePositive
	// Misdeclared: the verifier rejected the violation, but not through
	// the test the spec names.
	Misdeclared
	// Satisfied: obligation A -- the real solution does what the row
	// requires.
	Satisfied
	// Violated: obligation A -- the real solution does NOT do what the row
	// requires. This is a defect in the solution, not in the frozen spec,
	// which is the axiom.
	Violated
)

func (v Verdict) String() string {
	switch v {
	case Enforced:
		return "enforced"
	case FalsePositive:
		return "FALSE POSITIVE"
	case Misdeclared:
		return "misdeclared"
	case Satisfied:
		return "satisfied"
	case Violated:
		return "VIOLATED"
	}
	return "inconclusive"
}

// Result is one obligation's outcome.
type Result struct {
	Obligation Obligation
	Verdict    Verdict
	Detail     string
}

func (r Result) String() string {
	var parts []string
	for k, v := range r.Obligation.Combo {
		parts = append(parts, k+"="+v)
	}
	combo := strings.Join(parts, " ")
	return fmt.Sprintf("%s: %s — %s (%s)", r.Obligation.Section, combo, r.Verdict, r.Detail)
}

// Check discharges one obligation, always restoring the source.
func Check(task Task, ob Obligation) (result Result, resultErr error) {
	if ob.Unauthored() {
		return Result{ob, Inconclusive, "no violation authored for this obligation"}, nil
	}
	path := task.SourceRoot + "/" + ob.Violation.File
	original, err := readSource(task, path)
	if err != nil {
		return Result{ob, Inconclusive, "could not read source"}, err
	}
	if !strings.Contains(original, ob.Violation.Cut) {
		return Result{ob, Inconclusive, "cut text not present in source"}, nil
	}
	if strings.TrimSpace(task.TestCommand) == "" {
		return Result{ob, Inconclusive, "no verifier command declared"}, nil
	}

	// A red baseline cannot establish that the targeted violation caused a
	// rejection. Without this gate, an unrelated pre-existing failure is
	// attributed to the declared test and becomes a false Enforced verdict.
	// Baseline health is proof-critical even when an outer pipeline normally
	// checks it, because Check is also a public standalone operation.
	if passed, out := verifierPasses(task, nil, "", 0); !passed {
		return Result{ob, Inconclusive,
			fmt.Sprintf("baseline verifier did not pass (%s)", lastLine(out))}, nil
	}

	// Both the exit status AND the output matter. Comparing status alone
	// makes two different failures look identical -- the original raising
	// "wrap-around" and the mutant raising "field produced no values" both
	// exit 1, so every raising obligation reported itself unviolated.
	beforeCode, beforeOut := run(task, ob.Violation.Witness, task.SourceRoot)
	before := fmt.Sprintf("%d\n%s", beforeCode, beforeOut)

	broken := strings.Replace(original, ob.Violation.Cut, ob.Violation.With, 1)
	if err := writeSource(task, path, broken); err != nil {
		return Result{ob, Inconclusive, "could not write source"}, err
	}
	defer func() {
		if err := writeSource(task, path, original); err != nil {
			result = Result{ob, Inconclusive, "could not restore source"}
			resultErr = fmt.Errorf("enforce: restore source: %w", err)
		}
	}()

	afterCode, afterOut := run(task, ob.Violation.Witness, task.SourceRoot)
	after := fmt.Sprintf("%d\n%s", afterCode, afterOut)
	// A witness is optional. When one is authored it is the strongest
	// evidence the edit really violates the row; when the violation was
	// derived from the row's own required text, removing that statement is
	// a violation by construction and the declared test is the probe.
	if ob.Violation.Witness != "" && before == after {
		return Result{ob, Inconclusive,
			"witness behaved identically; the edit does not violate this obligation"}, nil
	}

	passed, out := verifierPasses(task, nil, "", 0)
	if passed {
		return Result{ob, FalsePositive,
			fmt.Sprintf("violated, yet the verifier still passed (%s)", lastLine(out))}, nil
	}

	// The suite rejected it -- but isolation requires that the DECLARED
	// test is what rejected it. Anything else means the obligation is
	// being caught incidentally, by a rule that is not the one named.
	if ob.Test == "" {
		return Result{ob, Misdeclared, "verifier rejected it, but the spec names no test"}, nil
	}
	if task.OneTest == "" {
		if strings.Contains(out, ob.Test) {
			return Result{ob, Enforced, "declared test appears among the failures"}, nil
		}
		return Result{ob, Misdeclared,
			fmt.Sprintf("verifier rejected it, but %q is not among the failures", ob.Test)}, nil
	}
	single, _ := run(task, strings.ReplaceAll(task.OneTest, "{test}", ob.Test), task.TestCwd)
	if single != 0 {
		return Result{ob, Enforced, fmt.Sprintf("%s fails on the violating solution", ob.Test)}, nil
	}
	return Result{ob, Misdeclared,
		fmt.Sprintf("%s still passes; something else rejected the violation", ob.Test)}, nil
}

// ListTests runs the task's TestList command and returns the names it
// printed. Returns nil when the task declares none, so the caller falls
// back to scanning a local directory.
func ListTests(task Task) map[string]bool {
	if task.TestList == "" {
		return nil
	}
	code, out := run(task, task.TestList, task.TestCwd)
	if code != 0 && out == "" {
		return nil
	}
	names := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if n := strings.TrimSpace(line); n != "" {
			names[n] = true
		}
	}
	return names
}

// witnessExercises reports whether breaking the obligation changes what
// the witness observes. A witness blind to its own rule is evidence about
// nothing, in either direction, so both obligations gate on this.
//
// Always restores the source.
func witnessExercises(task Task, ob Obligation) (bool, error) {
	if ob.Unauthored() {
		return false, nil
	}
	path := task.SourceRoot + "/" + ob.Violation.File
	original, err := readSource(task, path)
	if err != nil {
		return false, err
	}
	if !strings.Contains(original, ob.Violation.Cut) {
		return false, nil
	}
	beforeCode, beforeOut := run(task, ob.Violation.Witness, task.SourceRoot)
	if err := writeSource(task, path, strings.Replace(original, ob.Violation.Cut, ob.Violation.With, 1)); err != nil {
		return false, err
	}
	defer func() { _ = writeSource(task, path, original) }()
	afterCode, afterOut := run(task, ob.Violation.Witness, task.SourceRoot)
	return beforeCode != afterCode || beforeOut != afterOut, nil
}

// CheckAll discharges every obligation in order.
func CheckAll(task Task, obs []Obligation) ([]Result, error) {
	results := make([]Result, 0, len(obs))
	for _, ob := range obs {
		r, err := Check(task, ob)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	return results, nil
}

// verifierPasses asks the task's verifier. When a line map says which
// tests touch the changed line, only those run -- two seconds instead of
// forty-one. The full verifier still runs when there is no map, and when
// the selected tests pass, since a green subset is not yet a green suite.
func verifierPasses(task Task, lines LineMap, file string, line int) (bool, string) {
	if sel := SelectCommand(task.OneTest, mustRun(lines, file, line)); sel != "" {
		if code, out := run(task, sel, task.TestCwd); code != 0 {
			// A selected test already rejected it; the full suite cannot
			// un-reject it, so there is nothing more to learn.
			return false, out
		}
	}
	code, out := run(task, task.TestCommand, task.TestCwd)
	if task.PassFile == "" {
		return code == 0, out
	}
	body, err := readSource(task, task.PassFile)
	if err != nil {
		return false, out
	}
	return strings.TrimSpace(body) == task.PassValue, out
}

// mustRun returns the tests that execute a line, or nil when the map has
// nothing for that file.
func mustRun(lines LineMap, file string, line int) []string {
	if lines == nil {
		return nil
	}
	tests, measured := lines.TestsForLine(file, line)
	if !measured {
		return nil
	}
	return tests
}

func readSource(task Task, path string) (string, error) {
	if task.ReadFile == "" {
		b, err := os.ReadFile(path)
		return string(b), err
	}
	code, out := run(task, strings.ReplaceAll(task.ReadFile, "{file}", path), "")
	if code != 0 {
		return "", fmt.Errorf("read_file failed for %s", path)
	}
	return out, nil
}

func writeSource(task Task, path, text string) error {
	if task.WriteFile == "" {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
		purgeBytecode(task.SourceRoot)
	} else {
		cmd := exec.Command("sh", "-c", strings.ReplaceAll(task.WriteFile, "{file}", path))
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	if task.AfterWrite != "" {
		run(task, task.AfterWrite, "")
	}
	return nil
}

func run(task Task, command, cwd string) (int, string) {
	if command == "" {
		return 0, ""
	}
	cmd := exec.Command("sh", "-c", command)
	// A path inside a container is not a valid working directory here;
	// such commands carry their own context.
	if cwd != "" {
		if info, err := os.Stat(cwd); err == nil && info.IsDir() {
			cmd.Dir = cwd
		}
	}
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	return code, strings.TrimSpace(string(out))
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// purgeBytecode removes cached bytecode beneath root.
//
// PYTHONDONTWRITEBYTECODE stops NEW .pyc files being written; it does not
// stop an existing one from being loaded. A fixture that has been run
// before therefore keeps serving the previous source, the edit silently
// never takes effect, and every obligation reports "witness behaved
// identically" -- an all-clear produced by a caching artefact. Caught by
// exactly that symptom on the cron fixture.
//
// For source that is not local, the same job is the task's AfterWrite
// hook, since hyperray cannot reach into a container from here.
func purgeBytecode(root string) {
	if root == "" {
		return
	}
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == "__pycache__" {
			os.RemoveAll(path)
			return filepath.SkipDir
		}
		return nil
	})
}
