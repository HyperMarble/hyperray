// Package mutate implements hyperray's mutation pass: it asks whether the
// task's own test suite actually verifies each requirement, or whether
// it would stay green while the behaviour changed.
//
// This is the check that targets the false positive directly. An agent
// meets requirements A and B, skips C, and still passes -- because
// nothing in the suite ever tested C. Mutating the solution exposes
// that: break C, run the tests, and if they stay green, C is unverified.
//
// Two properties make this evidence rather than a suggestion:
//
//   - Exhaustive. The scope is one task's solution, so every mutation
//     point crossed with every operator is a small finite set. Nothing
//     is sampled or guessed, and the same input always yields the same
//     mutants.
//   - No false alarms. A surviving mutant is only reported once it has
//     been shown to genuinely behave differently, on real inputs. If it
//     behaves identically it is an equivalent mutant -- there is no
//     requirement being missed, and staying quiet is correct.
//
// The inputs used for that difference check are mechanical, never
// chosen by judgement: values harvested from the pinned dependency's own
// test suite (see internal/depharvest), plus boundary values derived
// from the constants already written in the solution -- which arrive for
// free, since a constant mutant n -> n+1 is itself the statement that n
// is a boundary worth testing.
package mutate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/HyperMarble/hyperray/internal/difftest"
)

// Mutant is one deliberate change to the solution.
type Mutant struct {
	ID       int    `json:"id"`
	Line     int    `json:"line"`
	Operator string `json:"operator"`
	Original string `json:"original"`
	Mutated  string `json:"mutated"`
	Source   string `json:"source"`
}

// Verdict is what running the task's tests against a mutant established.
type Verdict string

const (
	// Killed: the tests failed, so this behaviour IS verified.
	Killed Verdict = "killed"
	// Gap: the tests passed AND the mutant genuinely behaves differently
	// -- a proven hole in the test suite.
	Gap Verdict = "gap"
	// Equivalent: the tests passed but the mutant behaves identically on
	// every input tried, so there is nothing for a test to catch.
	Equivalent Verdict = "equivalent"
	// Unchecked: the tests passed but no inputs were available to decide
	// whether the mutant differs. Deliberately distinct from Equivalent:
	// reporting "nothing to see" for a check that never ran is the
	// false confidence hyperray exists to prevent.
	Unchecked Verdict = "unchecked"
)

// Outcome pairs a mutant with what was learned about it.
type Outcome struct {
	Mutant  Mutant
	Verdict Verdict
	// Witness is a concrete input on which the mutant and the original
	// behaved differently. Its presence is what makes a Gap a proof
	// rather than a claim.
	Witness string
}

// Report is the result of one mutation run.
type Report struct {
	Total      int
	Killed     int
	Gaps       []Outcome
	Equivalent int
	Unchecked  int
	Duration   time.Duration
}

// Pass reports whether the test suite caught every genuine behaviour
// change. Gaps are the finding; equivalents are not.
func (r Report) Pass() bool { return len(r.Gaps) == 0 }

func generatorPath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("mutate: could not resolve generator path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..",
		"third_party", "mutate", "generate_mutants.py"), nil
}

// Generate enumerates every mutant of the solution file. Exhaustive over
// the operator set, deterministic, and ordered by source line.
func Generate(pythonPath, solutionPath, language string) ([]Mutant, error) {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	gen, err := generatorPath()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(pythonPath, gen, solutionPath, language)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("generate_mutants.py: %w: %s", err, stderr.String())
	}
	var out struct {
		Mutants []Mutant `json:"mutants"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parsing mutants: %w", err)
	}
	return out.Mutants, nil
}

// BoundaryInputs derives the values that expose each constant mutation.
// A mutant that changes n to n+1 can only be caught by an input at that
// boundary, so the mutant list is itself the statement of which values
// matter -- no separate extraction, and no guessing.
func BoundaryInputs(mutants []Mutant) []any {
	seen := map[string]bool{}
	var out []any
	for _, m := range mutants {
		if m.Operator != "constant" {
			continue
		}
		for _, v := range []string{m.Original, m.Mutated} {
			if seen[v] {
				continue
			}
			seen[v] = true
			var n json.Number = json.Number(v)
			if f, err := n.Float64(); err == nil {
				out = append(out, f)
			}
		}
	}
	return out
}

// RunTests executes the task's own test command with the solution
// replaced by one mutant, then restores the original file.
//
// The original is restored through a deferred write that runs even on
// panic, and the restore is verified -- leaving a task's solution
// mutated on disk would be far worse than any missed finding.
func RunTests(solutionPath, mutantSource string, testCmd []string, workDir string,
	timeout time.Duration) (passed bool, err error) {

	original, err := os.ReadFile(solutionPath)
	if err != nil {
		return false, fmt.Errorf("mutate: reading solution: %w", err)
	}
	info, err := os.Stat(solutionPath)
	if err != nil {
		return false, err
	}

	defer func() {
		if restoreErr := os.WriteFile(solutionPath, original, info.Mode()); restoreErr != nil {
			err = fmt.Errorf("mutate: FAILED TO RESTORE %s: %w (original content was %d bytes)",
				solutionPath, restoreErr, len(original))
			return
		}
		if check, readErr := os.ReadFile(solutionPath); readErr != nil || !bytes.Equal(check, original) {
			err = fmt.Errorf("mutate: solution at %s did not restore correctly", solutionPath)
		}
	}()

	if err := os.WriteFile(solutionPath, []byte(mutantSource), info.Mode()); err != nil {
		return false, fmt.Errorf("mutate: writing mutant: %w", err)
	}

	// Python caches compiled bytecode next to the source. Rewriting the
	// file for each mutant can leave a stale .pyc that the interpreter
	// keeps serving after the original is restored -- which silently
	// evaluates every later mutant against an EARLIER mutant's code and
	// makes the whole pass untrustworthy. This was observed: a restored,
	// correct solution behaved like a previous mutant.
	//
	// Both halves are needed: purge what is already cached, and stop new
	// bytecode being written during the run.
	purgePycache(filepath.Dir(solutionPath))

	if len(testCmd) == 0 {
		return false, fmt.Errorf("mutate: no test command given")
	}
	cmd := exec.Command(testCmd[0], testCmd[1:]...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	var sink bytes.Buffer
	cmd.Stdout = &sink
	cmd.Stderr = &sink

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("mutate: starting tests: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case runErr := <-done:
		// Exit 0 means the suite passed with the mutant in place, i.e.
		// the mutant SURVIVED -- the interesting case.
		return runErr == nil, nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		// A mutant that hangs the suite is not a silent pass; treat it as
		// killed so it is never mistaken for a gap.
		return false, nil
	}
}

// purgePycache removes Python's compiled-bytecode caches under dir.
// Stale bytecode from a previous mutant is the difference between a
// trustworthy mutation result and a meaningless one, so failures here
// are ignored deliberately: the accompanying PYTHONDONTWRITEBYTECODE
// setting prevents new caches regardless, and a missing cache directory
// is the normal case rather than an error.
func purgePycache(dir string) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == "__pycache__" {
			_ = os.RemoveAll(path)
			return filepath.SkipDir
		}
		return nil
	})
}

// FunctionsByLine maps every line of a Python solution to the function
// that encloses it, so the equivalence check can call the function a
// mutant actually lives in.
//
// An earlier version called the FIRST top-level function in the file,
// which a real pluto task exposed as wrong in general: 74 of its 75
// mutants sat in `_parse_atom`, `parse_cron` and module scope, while the
// check kept calling `_parse_int`. Comparing a function no mutant touches
// proves nothing about that mutant.
//
// A method is reported as "Class.method": it needs an instance to call,
// which the caller must decide how to supply, and reporting the name is
// what lets it detect that rather than silently comparing the wrong thing.
func FunctionsByLine(pythonPath, solutionPath string) (map[int]string, error) {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	const src = `import ast,sys,json
tree=ast.parse(open(sys.argv[1]).read())
owner={}
for node in ast.walk(tree):
    if isinstance(node,ast.ClassDef):
        for s in node.body:
            if isinstance(s,(ast.FunctionDef,ast.AsyncFunctionDef)):
                for ln in range(s.lineno,(s.end_lineno or s.lineno)+1):
                    owner[ln]=node.name+"."+s.name
for n in tree.body:
    if isinstance(n,(ast.FunctionDef,ast.AsyncFunctionDef)):
        for ln in range(n.lineno,(n.end_lineno or n.lineno)+1):
            owner.setdefault(ln,n.name)
print(json.dumps(owner))`
	cmd := exec.Command(pythonPath, "-c", src, solutionPath)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("mutate: mapping lines to functions: %w: %s", err, errb.String())
	}
	raw := map[string]string{}
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("mutate: parsing line map: %w", err)
	}
	byLine := make(map[int]string, len(raw))
	for k, v := range raw {
		var n int
		if _, err := fmt.Sscanf(k, "%d", &n); err == nil {
			byLine[n] = v
		}
	}
	return byLine, nil
}

// DiffersFromOriginal reports whether a mutant genuinely behaves
// differently from the original on any of the given inputs, and returns
// the first input on which it does.
//
// This is what separates a real finding from a false alarm. A mutant
// that survives the tests is only a gap if it actually changed
// behaviour; if it cannot differ on any input, no test could have caught
// it and reporting it would waste the agent's time. Observed on a real
// fixture: `x < 0` mutated to `x < 1` survives every test, yet differs
// only at x == 0 -- which an earlier branch already handles, so the
// mutant is equivalent and correctly stays unreported.
//
// A false second return with a nil error means "no difference found on
// these inputs", which is weaker than "no difference exists"; that gap
// is why boundary inputs are derived from the mutants themselves.
func DiffersFromOriginal(pythonPath, originalSrc, mutantSrc, fnName string,
	inputs []any) (bool, string, error) {

	if len(inputs) == 0 {
		return false, "", fmt.Errorf("mutate: no inputs to compare on")
	}
	args := make([][]any, 0, len(inputs))
	for _, in := range inputs {
		args = append(args, []any{in})
	}

	res, err := difftest.Run(pythonPath, originalSrc, fnName, mutantSrc, fnName, args)
	if err != nil {
		return false, "", err
	}
	if len(res.Disagreements) > 0 {
		return true, fmt.Sprintf("%v", res.Disagreements[0].Input), nil
	}

	// Agreement is only evidence of equivalence if the function actually
	// RAN. A real sktime run exposed this: the inputs were single values
	// while the function took two parameters, so every call raised
	// TypeError on both sides, they "agreed", and 13 mutants were
	// reported equivalent when none had been exercised at all. That is a
	// confident wrong answer standing in for an honest "unknown" -- the
	// exact false confidence hyperray exists to prevent, reproduced inside hyperray.
	//
	// So: if nothing ever returned a value, the comparison was vacuous.
	// Report that rather than silently calling it equivalence.
	if res.Agreements > 0 && !anyInputProducedAValue(pythonPath, originalSrc, fnName, args) {
		return false, "", fmt.Errorf(
			"mutate: comparison was vacuous -- %s never returned a value on any of the "+
				"%d inputs (wrong arity or unsupported argument types), so agreement "+
				"proves nothing", fnName, len(args))
	}
	return false, "", nil
}

// anyInputProducedAValue reports whether the function under test returned
// normally for at least one input. If it only ever raised, the inputs did
// not fit its signature and no conclusion about equivalence is available.
func anyInputProducedAValue(pythonPath, src, fnName string, args [][]any) bool {
	// Comparing the source against itself: every input agrees by
	// construction, so the only information sought is whether the calls
	// returned values or raised.
	res, err := difftest.Run(pythonPath, src, fnName, src, fnName, args)
	if err != nil {
		return false
	}
	return res.ReturnedNormally > 0
}
