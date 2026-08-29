package tests

import (
	"os"
	"os/exec"
	"testing"

	"github.com/HyperMarble/hyperray/internal/enforce"
	"github.com/HyperMarble/hyperray/internal/mutate"
)

// probe runs the real entry point on one input and prints what came back,
// so an adversary's behaviour can be compared against the real one's.
// Nothing here encodes what the answer SHOULD be -- the reference solution
// is the oracle, so any difference is a deviation from correct.
func probe(expr string) enforce.Probe {
	return enforce.Probe{
		Name: expr,
		Command: "python3 -c \"from cronfield.parser import parse_cron; " +
			"e = parse_cron('" + expr + "'); print(sorted(e.minutes), sorted(e.hours))\" 2>&1",
	}
}

// The whole mechanism, end to end, with no rule per kind of requirement
// and no guessed inputs: generate adversaries mechanically, ask the solver
// which ones really behave differently, then ask the task's own verifier
// about those.
//
// An adversary that deviates and still passes is a proven false positive:
// a wrong solution the task cannot reject.
func TestDiscover_FindsFalsePositivesWithoutPerCaseRules(t *testing.T) {
	if _, err := os.Stat(cronRoot + "/app/cronfield/parser.py"); err != nil {
		t.Skip("cron fixture not present")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}

	task := cronTask(t)
	task.SourceRoot = cronRoot + "/app"

	solution := cronRoot + "/app/cronfield/parser.py"
	before, err := os.ReadFile(solution)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	mutants, err := mutate.Generate("python3", solution, "python")
	if err != nil {
		t.Skipf("adversary generation unavailable: %v", err)
	}
	if len(mutants) == 0 {
		t.Fatal("no adversaries generated")
	}
	// A bounded slice keeps the test quick; the mechanism is identical for
	// the full set, which is what `hyperray check` runs.
	// Probes must reach beyond what the task's own tests exercise. A probe
	// set drawn only from passing inputs can never expose a rule that only
	// fires on inputs nobody tests -- the same circularity that made
	// test-derived inputs useless.
	probes := []enforce.Probe{
		probe("*/15 * * * *"), probe("0-59 * * * *"), probe("10-40/15 * * * *"),
		probe("5 3 * * *"), probe("0-99 * * * *"), probe("30-10 * * * *"),
		probe("1,,2 * * * *"), probe("5/2 * * * *"), probe("59 23 31 12 6"),
		probe("0-0 0-0 1-1 1-1 0-0"),
	}

	found, err := enforce.Discover(task, "cronfield/parser.py", mutants, probes)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	after, err := os.ReadFile(solution)
	if err != nil {
		t.Fatalf("re-reading fixture: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("solution was not restored")
	}

	for _, d := range found {
		if !d.Checked {
			t.Errorf("reported a deviation that was never demonstrated: %+v", d)
		}
		if d.Input == "" {
			t.Errorf("reported a deviation with no witness input: %+v", d)
		}
	}
	t.Logf("adversaries=%d proven false positives=%d", len(mutants), len(found))
	for _, d := range found {
		t.Logf("  L%d %q->%q witness %s", d.Mutant.Line, d.Mutant.Original, d.Mutant.Mutated, d.Input)
	}
}

// An adversary is only evidence when it has been SHOWN to deviate. With no
// probe there is nothing to show it with, so the run must refuse rather
// than quietly report nothing found.
func TestDiscover_RefusesWithoutProbes(t *testing.T) {
	task := cronTask(t)
	_, err := enforce.Discover(task, "cronfield/parser.py",
		[]mutate.Mutant{{ID: 1, Source: "x = 1"}}, nil)
	if err == nil {
		t.Fatal("want an error when there is no probe to demonstrate deviation with")
	}
}
