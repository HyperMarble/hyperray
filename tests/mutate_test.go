package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HyperMarble/ray/internal/mutate"
)

// mutDemo builds a task whose tests deliberately under-cover it, matching
// the platform's own description of a false positive: the prompt states
// requirements A, B and C, the tests only check A and B, so an agent that
// skips C still passes.
//
//	A -> x > 100 returns "big"      (tested)
//	B -> x == 0  returns "zero"     (tested)
//	C -> x < 0   returns "negative" (NOT tested)
func mutDemo(t *testing.T) (solution, workDir string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	solution = filepath.Join(dir, "solution.py")
	if err := os.WriteFile(solution, []byte(`def classify(x):
    if x > 100:
        return "big"
    if x == 0:
        return "zero"
    if x < 0:
        return "negative"
    return "small"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := `import sys; sys.path.insert(0, ` + "`" + `DIR` + "`" + `)
from solution import classify

def test_big():   assert classify(500) == "big"
def test_zero():  assert classify(0) == "zero"
def test_small(): assert classify(50) == "small"
`
	tests = replaceAll(tests, "`DIR`", `"`+dir+`"`)
	if err := os.WriteFile(filepath.Join(dir, "tests", "test_classify.py"), []byte(tests), 0o644); err != nil {
		t.Fatal(err)
	}
	return solution, dir
}

func replaceAll(s, old, new string) string {
	out := ""
	for {
		i := indexOf(s, old)
		if i < 0 {
			return out + s
		}
		out += s[:i] + new
		s = s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestMutate_FindsUntestedRequirement is the mutation pass's own
// load-bearing test. It does not check that mutants can be generated --
// it checks that running them against a real, under-covering test suite
// exposes the untested requirement.
//
// Measured on this fixture: 13 mutants, 7 killed, 6 survived. Four of the
// survivors sit on the `x < 0` line -- requirement C, which no test
// touches. The other two are `100 -> 99` and `100 -> 101`: the tests pass
// 500 and 50, so nothing ever probes the boundary. Both are real gaps,
// found mechanically rather than suggested.
func TestMutate_FindsUntestedRequirement(t *testing.T) {
	genPython := testPython3(t) // needs tree_sitter
	runPython := testHarvestPython(t)
	solution, workDir := mutDemo(t)

	testCmd := []string{runPython, "-m", "pytest", "tests/", "-q", "-p", "no:cacheprovider"}

	mutants, err := mutate.Generate(genPython, solution, "python")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(mutants) == 0 {
		t.Fatal("no mutants generated from a function full of operators and constants")
	}

	killed := 0
	survivedOnLine := map[int]int{}
	for _, m := range mutants {
		passed, err := mutate.RunTests(solution, m.Source, testCmd, workDir, 60*time.Second)
		if err != nil {
			t.Fatalf("mutant %d (L%d %s): %v", m.ID, m.Line, m.Operator, err)
		}
		if passed {
			survivedOnLine[m.Line]++
		} else {
			killed++
		}
	}

	if killed == 0 {
		t.Fatal("no mutant was killed -- the test suite is not running at all, so a " +
			"survivor would prove nothing")
	}
	// Line 6 is `if x < 0:` -- the requirement the suite never checks.
	if survivedOnLine[6] == 0 {
		t.Fatalf("no mutant survived on the untested requirement's line; mutation failed to "+
			"find a gap that is definitely there (survivors by line: %v)", survivedOnLine)
	}

	// The original must be back on disk exactly as it started, or ray has
	// damaged the task it was asked to verify.
	after, err := os.ReadFile(solution)
	if err != nil {
		t.Fatal(err)
	}
	if indexOf(string(after), `if x < 0:`) < 0 {
		t.Fatal("the solution was not restored after mutation")
	}

	t.Logf("mutants=%d killed=%d survivors_by_line=%v", len(mutants), killed, survivedOnLine)
}

// TestMutate_BoundaryInputsComeFromTheMutants confirms the inputs that
// expose a constant mutation are derived from the mutants themselves,
// not chosen by judgement: a mutant that changes 100 to 101 is itself the
// statement that 99/100/101 are the values worth testing.
func TestMutate_BoundaryInputsComeFromTheMutants(t *testing.T) {
	genPython := testPython3(t)
	solution, _ := mutDemo(t)

	mutants, err := mutate.Generate(genPython, solution, "python")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got := mutate.BoundaryInputs(mutants)
	if len(got) == 0 {
		t.Fatal("no boundary inputs derived from a solution containing constants")
	}
	want := map[float64]bool{100: false, 99: false, 101: false, 0: false, 1: false, -1: false}
	for _, v := range got {
		if f, ok := v.(float64); ok {
			if _, tracked := want[f]; tracked {
				want[f] = true
			}
		}
	}
	for value, found := range want {
		if !found {
			t.Errorf("boundary %v was not derived, so the mutation that needs it could not be exposed", value)
		}
	}
}
