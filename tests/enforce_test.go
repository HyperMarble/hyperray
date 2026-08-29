package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/enforce"
)

// The cron fixture is a real task with a real, passing test suite. These
// tests need it present; they skip rather than fail when it is not, so
// the suite stays runnable on a clean checkout.
const cronRoot = "/tmp/crontask"

func cronTask(t *testing.T) enforce.Task {
	t.Helper()
	if _, err := os.Stat(cronRoot + "/app/cronfield/parser.py"); err != nil {
		t.Skip("cron fixture not present")
	}
	return enforce.Task{
		SourceRoot:  cronRoot + "/app",
		TestCwd:     cronRoot,
		TestCommand: "PYTHONPATH=" + cronRoot + "/app python3 -m pytest -q --no-header -p no:cacheprovider tests",
		OneTest:     "PYTHONPATH=" + cronRoot + "/app python3 -m pytest -q --no-header -p no:cacheprovider tests -k {test}",
	}
}

// isolatedEnforcementTask models the measured cron failure without sharing
// /tmp/crontask with discovery and end-to-end tests. The declared wrap test
// checks only the exception type, so deleting the wrap guard still passes via
// the independent empty-range guard. The step test contrasts it by rejecting
// a deleted step guard.
func isolatedEnforcementTask(t *testing.T) (enforce.Task, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"parser.py": `class CronExprError(ValueError):
    pass

def parse_range(a, b):
    if a > b:
        raise CronExprError(f"wrap-around range {a}-{b} not supported")
    # Range expansion.
    values = set(range(a, b + 1))
    if not values:
        raise CronExprError("field produced no values")
    return values

def parse_step(step):
    if step <= 0:
        raise CronExprError(f"step must be positive, got {step}")
    return 10 // step
`,
		"verify.py": `from parser import CronExprError, parse_range, parse_step

def expect_cron_error(call):
    try:
        call()
    except CronExprError:
        return
    raise AssertionError("expected CronExprError")

expect_cron_error(lambda: parse_range(5, 3))
expect_cron_error(lambda: parse_step(0))
`,
		"one_test.py": `import sys
from parser import CronExprError, parse_range, parse_step

def expect_cron_error(call):
    try:
        call()
    except CronExprError:
        return
    raise AssertionError("expected CronExprError")

name = sys.argv[1]
if name == "test_wrap_around_range_raises":
    expect_cron_error(lambda: parse_range(5, 3))
elif name == "test_zero_step_raises":
    expect_cron_error(lambda: parse_step(0))
else:
    raise SystemExit(2)
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write isolated enforcement fixture %s: %v", name, err)
		}
	}
	return enforce.Task{
		SourceRoot:  root,
		TestCwd:     root,
		TestCommand: "python3 verify.py",
		OneTest:     "python3 one_test.py {test}",
	}, filepath.Join(root, "parser.py")
}

func rangeWitness(a, b int) string {
	return "python3 -c \"from parser import parse_range; print(sorted(parse_range(" +
		fmt.Sprint(a) + ", " + fmt.Sprint(b) + ")))\" 2>&1"
}

func stepWitness(step int) string {
	return "python3 -c \"from parser import parse_step; print(parse_step(" +
		fmt.Sprint(step) + "))\" 2>&1"
}

// A declared test that exists, names the right rule, and still does not
// enforce it. Deleting the wrap-around guard leaves range(30, 11) empty,
// so a DIFFERENT rule -- "field produced no values" -- raises the same
// exception type and test_wrap_around_range_raises stays green. Verified
// by hand before this test was written: the violating solution passes the
// entire suite.
//
// This is the case the declarative half of layer 2 gets wrong, and the
// whole reason obligation B needs execution rather than a naming
// convention.
func TestEnforce_DeclaredTestDoesNotActuallyEnforce(t *testing.T) {
	task, source := isolatedEnforcementTask(t)
	ob := enforce.Obligation{
		Section: "3. Head resolution",
		Combo:   map[string]string{"head_form": "range", "bounds": "reversed"},
		Test:    "test_wrap_around_range_raises",
		Violation: enforce.Violation{
			File:    "parser.py",
			Cut:     "if a > b:\n        raise CronExprError(f\"wrap-around range {a}-{b} not supported\")",
			With:    "pass",
			Witness: rangeWitness(30, 10),
		},
	}

	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	res, err := enforce.Check(task, ob)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Verdict != enforce.FalsePositive {
		t.Errorf("got %v (%s), want FALSE POSITIVE — the declared test does not enforce this rule",
			res.Verdict, res.Detail)
	}

	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("re-reading fixture: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("source was not restored")
	}
}

// The contrasting case: a rule whose declared test really does fail when
// the rule is broken. Without this, the test above would pass for a
// tool that simply reported FALSE POSITIVE unconditionally.
func TestEnforce_DeclaredTestReallyEnforces(t *testing.T) {
	task, _ := isolatedEnforcementTask(t)
	ob := enforce.Obligation{
		Section: "2. Step",
		Combo:   map[string]string{"step_present": "yes", "step_value": "non-positive"},
		Test:    "test_zero_step_raises",
		Violation: enforce.Violation{
			File:    "parser.py",
			Cut:     "if step <= 0:\n        raise CronExprError(f\"step must be positive, got {step}\")",
			With:    "pass",
			Witness: stepWitness(0),
		},
	}
	res, err := enforce.Check(task, ob)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Verdict != enforce.Enforced {
		t.Errorf("got %v (%s), want enforced", res.Verdict, res.Detail)
	}
}

// An edit that changes nothing must never be read as enforcement. This is
// the guard against the failure mode that produced nine wrong verdicts
// earlier: treating "no difference observed" as a positive result.
func TestEnforce_UndemonstrableViolationIsInconclusive(t *testing.T) {
	task, _ := isolatedEnforcementTask(t)
	ob := enforce.Obligation{
		Section: "2. Step",
		Combo:   map[string]string{"step_present": "no"},
		Test:    "test_single_integer_field",
		Violation: enforce.Violation{
			File:    "parser.py",
			Cut:     "# Range expansion.",
			With:    "# Expand the range.",
			Witness: rangeWitness(1, 2),
		},
	}
	res, err := enforce.Check(task, ob)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Verdict != enforce.Inconclusive {
		t.Errorf("got %v (%s), want inconclusive", res.Verdict, res.Detail)
	}
}

// A pre-existing red suite is not evidence that a targeted violation was
// rejected. Check must stop before editing and return inconclusive.
func TestEnforce_RedBaselineIsInconclusive(t *testing.T) {
	task, source := isolatedEnforcementTask(t)
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	red := strings.Replace(string(before),
		"if step <= 0:\n        raise CronExprError(f\"step must be positive, got {step}\")",
		"if step <= 0:\n        pass", 1)
	if err := os.WriteFile(source, []byte(red), 0o644); err != nil {
		t.Fatal(err)
	}

	ob := enforce.Obligation{
		Section: "3. Head resolution",
		Combo:   map[string]string{"head_form": "range", "bounds": "reversed"},
		Test:    "test_wrap_around_range_raises",
		Violation: enforce.Violation{
			File:    "parser.py",
			Cut:     "if a > b:\n        raise CronExprError(f\"wrap-around range {a}-{b} not supported\")",
			With:    "pass",
			Witness: rangeWitness(30, 10),
		},
	}
	res, err := enforce.Check(task, ob)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Verdict != enforce.Inconclusive || !strings.Contains(res.Detail, "baseline verifier did not pass") {
		t.Fatalf("got %v (%s), want inconclusive red-baseline finding", res.Verdict, res.Detail)
	}
	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != red {
		t.Fatal("red-baseline check edited the source")
	}
}
