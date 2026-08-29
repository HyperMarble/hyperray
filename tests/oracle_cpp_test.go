package tests

import (
	"os"
	"os/exec"
	"testing"

	"github.com/HyperMarble/hyperray/internal/oracle"
)

// testEsbmcPath locates the esbmc binary: RAY_ESBMC_PATH env var, then
// whatever "esbmc" resolves to on PATH. Skips the calling test if neither
// is available -- esbmc isn't bundled yet (v0.1.0), same posture as pict
// and the touchstone-patch venv.
func testEsbmcPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("RAY_ESBMC_PATH"); p != "" {
		return p
	}
	if p, err := exec.LookPath("esbmc"); err == nil {
		return p
	}
	t.Skip("esbmc binary not found; brew install esbmc, or set RAY_ESBMC_PATH")
	return ""
}

func TestOracleCPP_PlainProof(t *testing.T) {
	esbmc := testEsbmcPath(t)
	src := `#include <assert.h>
int clamp(int x, int lo, int hi) {
    if (x < lo) return lo;
    if (x > hi) return hi;
    return x;
}
int main() {
    int x, lo, hi;
    __ESBMC_assume(lo <= hi);
    int r = clamp(x, lo, hi);
    assert(r >= lo && r <= hi);
    return 0;
}
`
	v, err := oracle.ProveCPP(esbmc, src, 5)
	if err != nil {
		t.Fatalf("ProveCPP: %v", err)
	}
	if v.Status != "PROVED" {
		t.Fatalf("got status %q, want PROVED: %+v", v.Status, v)
	}
}

func TestOracleCPP_PlainRefutation(t *testing.T) {
	esbmc := testEsbmcPath(t)
	src := `#include <assert.h>
int broken_clamp(int x, int lo, int hi) {
    if (x < lo) return lo;
    return x;
}
int main() {
    int x, lo, hi;
    __ESBMC_assume(lo <= hi);
    int r = broken_clamp(x, lo, hi);
    assert(r >= lo && r <= hi);
    return 0;
}
`
	v, err := oracle.ProveCPP(esbmc, src, 5)
	if err != nil {
		t.Fatalf("ProveCPP: %v", err)
	}
	if v.Status != "REFUTED" {
		t.Fatalf("got status %q, want REFUTED: %+v", v.Status, v)
	}
	if v.Counterexample == "" {
		t.Error("REFUTED verdict missing a counterexample")
	}
}
