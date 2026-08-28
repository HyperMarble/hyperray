package tests

import (
	"os"
	"os/exec"
	"testing"

	"github.com/HyperMarble/ray/internal/oracle"
)

// testVerusPath locates the verus binary: RAY_VERUS_PATH env var, then
// whatever "verus" resolves to on PATH. Skips the calling test if neither
// is available -- verus isn't bundled yet (v0.1.0), same posture as esbmc
// and the touchstone-patch venv.
func testVerusPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("RAY_VERUS_PATH"); p != "" {
		return p
	}
	if p, err := exec.LookPath("verus"); err == nil {
		return p
	}
	t.Skip("verus binary not found; set RAY_VERUS_PATH to a downloaded verus release")
	return ""
}

func TestOracleRust_PlainProof(t *testing.T) {
	verus := testVerusPath(t)
	src := `use vstd::prelude::*;

verus! {

fn clamp(x: i32, lo: i32, hi: i32) -> (r: i32)
    requires lo <= hi,
    ensures lo <= r && r <= hi,
{
    if x < lo { lo }
    else if x > hi { hi }
    else { x }
}

}

fn main() {}
`
	v, err := oracle.ProveRust(verus, src)
	if err != nil {
		t.Fatalf("ProveRust: %v", err)
	}
	if v.Status != "PROVED" {
		t.Fatalf("got status %q, want PROVED: %+v", v.Status, v)
	}
}

func TestOracleRust_PlainRefutation(t *testing.T) {
	verus := testVerusPath(t)
	src := `use vstd::prelude::*;

verus! {

fn broken_clamp(x: i32, lo: i32, hi: i32) -> (r: i32)
    requires lo <= hi,
    ensures lo <= r && r <= hi,
{
    if x < lo { lo }
    else { x }
}

}

fn main() {}
`
	v, err := oracle.ProveRust(verus, src)
	if err != nil {
		t.Fatalf("ProveRust: %v", err)
	}
	if v.Status != "REFUTED" {
		t.Fatalf("got status %q, want REFUTED: %+v", v.Status, v)
	}
}

// TestOracleRust_MultiItemProofNotMisreadAsRefuted is the regression test
// for a real bug a Layer 3 stress test found: a source file with MORE THAN
// ONE verified item (here, a function plus a separate `proof fn`) prints
// "N verified, 0 errors" with N > 1, which the old exact-string match on
// "1 verified, 0 errors" missed entirely -- it fell through to a generic
// "errors" substring check, which also matches inside "0 errors", so a
// fully successful multi-item proof was misclassified as REFUTED.
func TestOracleRust_MultiItemProofNotMisreadAsRefuted(t *testing.T) {
	verus := testVerusPath(t)
	src := `use vstd::prelude::*;

verus! {

fn abs_diff(a: i32, b: i32) -> (r: i32)
    requires a - b > i32::MIN, b - a > i32::MIN,
    ensures r >= 0,
{
    if a >= b { a - b } else { b - a }
}

proof fn abs_diff_symmetric(a: int, b: int)
    ensures (if a >= b { a - b } else { b - a }) == (if b >= a { b - a } else { a - b }),
{
}

}

fn main() {}
`
	v, err := oracle.ProveRust(verus, src)
	if err != nil {
		t.Fatalf("ProveRust: %v", err)
	}
	if v.Status != "PROVED" {
		t.Fatalf("got status %q, want PROVED (multi-item proof misclassified): %+v", v.Status, v)
	}
}
