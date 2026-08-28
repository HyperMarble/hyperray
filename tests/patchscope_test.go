package tests

import (
	"testing"

	"github.com/HyperMarble/ray/internal/sufficiency"
)

// A unified diff, shaped like the ones real repo-modification tasks ship.
const samplePatch = `diff --git a/pkg/thing.py b/pkg/thing.py
--- a/pkg/thing.py
+++ b/pkg/thing.py
@@ -10,6 +10,9 @@ def existing():
     a = 1
     b = 2
+    if a > b:
+        raise ValueError("new rule")
+    c = 3
     return a

`

func TestChangedLines_TracksPostPatchNumbering(t *testing.T) {
	changed := sufficiency.ChangedLines(samplePatch)
	lines, ok := changed["thing.py"]
	if !ok {
		t.Fatalf("file not found in %v", changed)
	}
	// The hunk starts at new-file line 10; two context lines consume 10
	// and 11, so the additions land on 12, 13 and 14.
	for _, want := range []int{12, 13, 14} {
		if !lines[want] {
			t.Errorf("line %d should be marked changed, got %v", want, lines)
		}
	}
	for _, notWant := range []int{10, 11, 15, 16} {
		if lines[notWant] {
			t.Errorf("line %d is context or beyond, must not be marked changed", notWant)
		}
	}
}

// The bug this exists to prevent: on a real deep-swe task, sufficiency
// judged all 1000+ lines of numba's stencil.py and reported 19 pre-existing
// library errors as behaviour "outside the frozen contract". The frozen
// spec describes the TASK, so only the lines the task's patch introduced
// are in scope.
func TestScopeToPatch_DropsPreExistingOutcomes(t *testing.T) {
	outcomes := []sufficiency.Outcome{
		{Kind: "raise", Line: 5, SourceText: `raise ValueError("pre-existing")`},
		{Kind: "raise", Line: 13, SourceText: `raise ValueError("new rule")`},
		{Kind: "raise", Line: 900, SourceText: `raise ValueError("also pre-existing")`},
	}
	kept := sufficiency.ScopeToPatch(outcomes, "/anywhere/pkg/thing.py", samplePatch)
	if len(kept) != 1 {
		t.Fatalf("want only the patch's own outcome, got %d: %v", len(kept), kept)
	}
	if kept[0].Line != 13 {
		t.Errorf("kept the wrong outcome: %v", kept[0])
	}
}

// A greenfield task's solution file IS the task, and ships no patch.
// Narrowing on an absent patch would silently stop checking anything.
func TestScopeToPatch_NoPatchForThisFileKeepsEverything(t *testing.T) {
	outcomes := []sufficiency.Outcome{
		{Kind: "raise", Line: 5, SourceText: `raise ValueError("a")`},
		{Kind: "raise", Line: 9, SourceText: `raise ValueError("b")`},
	}
	kept := sufficiency.ScopeToPatch(outcomes, "/anywhere/other.py", samplePatch)
	if len(kept) != len(outcomes) {
		t.Fatalf("want all %d outcomes kept when the patch does not name this file, got %d",
			len(outcomes), len(kept))
	}
}
