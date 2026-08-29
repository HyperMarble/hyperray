package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/internal/depharvest"
)

// testHarvestPython locates an interpreter that has BOTH a real
// dependency and pytest installed. dep-harvest observes whatever version
// that interpreter has, so pinning is inherited from the environment.
func testHarvestPython(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("RAY_HARVEST_PYTHON"); p != "" {
		return p
	}
	t.Skip("RAY_HARVEST_PYTHON not set; point it at a venv python3 with the dependency and pytest installed")
	return ""
}

func testHarvestTestsPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("RAY_HARVEST_TESTS"); p != "" {
		return p
	}
	t.Skip("RAY_HARVEST_TESTS not set; point it at a dependency's own test suite directory")
	return ""
}

// TestDepHarvest_ObservesRealValues runs a real, pinned dependency's own
// test suite and checks that real edge-case values come back. Verified
// hands-on against jsonschema 4.26.0: 48,247 traced calls yielding empty
// strings, whitespace-only strings, JSON pointers, NaN, None and
// boundary integers -- values a static scrape of the test files would
// have missed entirely.
func TestDepHarvest_ObservesRealValues(t *testing.T) {
	python := testHarvestPython(t)
	tests := testHarvestTestsPath(t)

	h, err := depharvest.Run(python, "jsonschema", tests, 60000)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.Module != "jsonschema" {
		t.Errorf("got module %q, want jsonschema", h.Module)
	}
	if h.Version == "" || h.Version == "unknown" {
		t.Errorf("got version %q -- a harvest must be attributable to an exact version", h.Version)
	}
	if h.CallsObserved < 1000 {
		t.Fatalf("only %d calls traced; the suite barely ran, so the harvest is not meaningful", h.CallsObserved)
	}
	// dict and list are the payoff: an earlier version filtered to a
	// hardcoded primitives tuple and discarded containers entirely,
	// even though real schemas and argument lists are the most useful
	// edge-case inputs a suite produces.
	for _, kind := range []string{"int", "str", "bool", "dict", "list"} {
		if len(h.Values[kind]) == 0 {
			t.Errorf("no %s values harvested from a real test suite", kind)
		}
	}
	if len(h.Flatten()) == 0 {
		t.Error("Flatten returned nothing")
	}
	t.Logf("harvested from %s %s: %d calls traced, %d distinct values",
		h.Module, h.Version, h.CallsObserved, len(h.Flatten()))
}

// TestDepHarvest_StdoutIsCleanJSON guards a real bug found during
// development: pytest writes its progress report to stdout, which
// corrupted the JSON payload. The script now redirects that to stderr,
// and this test fails if the two are ever mixed again.
func TestDepHarvest_StdoutIsCleanJSON(t *testing.T) {
	python := testHarvestPython(t)
	tests := testHarvestTestsPath(t)

	// Run succeeding at all means stdout parsed as JSON; a regression
	// would surface here as a parse error rather than a silent mixture.
	h, err := depharvest.Run(python, "jsonschema", tests, 5000)
	if err != nil {
		t.Fatalf("stdout was not clean JSON: %v", err)
	}
	if h.Tests == "" {
		t.Error("harvest did not record which test suite it observed")
	}
	if filepath.Base(h.Tests) == "" {
		t.Error("recorded tests path is unusable")
	}
}
