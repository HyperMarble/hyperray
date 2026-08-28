// Package depharvest collects real edge-case values from a pinned
// dependency by running that dependency's own test suite and recording
// the values that actually flow through it.
//
// The point is to stop guessing what a tricky input looks like. A
// dependency's maintainers already found its edge cases and encoded them
// in its tests; executing that suite observes the real values -- empty
// strings, NaN, boundary integers -- including ones produced by fixtures
// and parametrized cases that no static scrape of the test files would
// ever see.
//
// Harvested values feed two places: `coverage`'s parameter value lists,
// and `difftest`'s concrete inputs. That second one matters most --
// diff-test's evidence is only as good as the inputs it runs, and a task's
// own test suite is exactly the place where blind spots already exist.
package depharvest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
)

// Harvest is the result of observing one pinned dependency's test run.
type Harvest struct {
	Module string `json:"module"`
	// Version is the exact installed version observed, so a harvest is
	// attributable and reproducible rather than floating.
	Version string `json:"version"`
	Tests   string `json:"tests"`
	// CallsObserved is how many calls into the dependency were traced --
	// a low number means the suite barely ran, so a thin result should
	// be read as "not much was observed", not "few edge cases exist".
	CallsObserved int `json:"calls_observed"`
	// Values holds observed values keyed by their Python type name --
	// "int", "str", "dict", "list", and whatever else actually appeared,
	// including a dependency's own namedtuples. The keys are not a fixed
	// set: whatever the real suite produced is what shows up.
	Values map[string][]any `json:"values"`
}

// harvestScriptPath resolves harvest_runtime.py relative to this source
// file so Run works regardless of the caller's working directory.
func harvestScriptPath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("depharvest: could not resolve harvest script path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..",
		"third_party", "dep-harvest", "harvest_runtime.py"), nil
}

// Run traces the dependency named by module while its own test suite at
// testsPath executes, and returns the primitive values observed entering
// its code.
//
// pythonPath must be an interpreter with that dependency AND pytest
// installed -- the version harvested is whatever that interpreter has,
// so pinning is inherited from the environment (a task's Dockerfile, or
// a locked venv) rather than re-declared here.
//
// maxCalls bounds tracing overhead on large suites; 0 uses the script's
// own default.
func Run(pythonPath, module, testsPath string, maxCalls int) (Harvest, error) {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	if _, err := exec.LookPath(pythonPath); err != nil {
		return Harvest{}, fmt.Errorf("python interpreter not found (%q): %w", pythonPath, err)
	}
	script, err := harvestScriptPath()
	if err != nil {
		return Harvest{}, err
	}

	args := []string{script, "--module", module, "--tests", testsPath}
	if maxCalls > 0 {
		args = append(args, "--max-calls", fmt.Sprintf("%d", maxCalls))
	}

	cmd := exec.Command(pythonPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	// The dependency's own test output goes to stderr by design so it
	// cannot corrupt the JSON on stdout; it is captured but not treated
	// as an error, since a failing test in the dependency does not
	// invalidate the values observed along every other path.
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Harvest{}, fmt.Errorf("harvest_runtime.py: %w: %s", err, tail(stderr.String()))
	}

	var h Harvest
	if err := json.Unmarshal(stdout.Bytes(), &h); err != nil {
		return Harvest{}, fmt.Errorf("parsing harvest output: %w", err)
	}
	return h, nil
}

// Flatten returns every harvested value as a single slice, suitable for
// use as diff-test inputs or coverage value lists.
//
// It iterates whatever kinds the harvest actually produced rather than a
// fixed list of expected ones -- a hardcoded list would silently drop
// any kind not anticipated, which is how the dict and list values (the
// most useful ones) went missing in an earlier version. Kinds are
// visited in sorted order so output is deterministic.
func (h Harvest) Flatten() []any {
	kinds := make([]string, 0, len(h.Values))
	for kind := range h.Values {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)

	var out []any
	for _, kind := range kinds {
		out = append(out, h.Values[kind]...)
	}
	return out
}

func tail(s string) string {
	const max = 400
	if len(s) <= max {
		return s
	}
	return "..." + s[len(s)-max:]
}
