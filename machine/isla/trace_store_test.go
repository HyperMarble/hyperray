// A stored trace must come back only for the same encoding under the same
// architecture, and a store with no root must keep nothing.
package isla

import (
	"testing"
)

func TestLookupFindsNothingBeforeAnythingIsKept(t *testing.T) {
	store := NewTraceStore(t.TempDir())
	if _, found := store.Lookup("d65f03c0", "arch-digest"); found {
		t.Fatal("Lookup() found a trace that was never kept")
	}
}

func TestAKeptTraceIsFoundAgain(t *testing.T) {
	store := NewTraceStore(t.TempDir())
	if err := store.Keep("d65f03c0", "arch-digest", InstructionTrace{TraceCount: 2}); err != nil {
		t.Fatalf("Keep() error = %v", err)
	}
	found, ok := store.Lookup("d65f03c0", "arch-digest")
	if !ok {
		t.Fatal("Lookup() did not find the kept trace")
	}
	if found.TraceCount != 2 {
		t.Fatalf("Lookup() TraceCount = %d, want 2", found.TraceCount)
	}
}

func TestADifferentArchitectureDoesNotShareTraces(t *testing.T) {
	store := NewTraceStore(t.TempDir())
	if err := store.Keep("d65f03c0", "one", InstructionTrace{TraceCount: 2}); err != nil {
		t.Fatalf("Keep() error = %v", err)
	}
	if _, found := store.Lookup("d65f03c0", "another"); found {
		t.Fatal("Lookup() returned a trace produced by a different architecture")
	}
}

func TestAStoreWithoutARootKeepsNothing(t *testing.T) {
	store := NewTraceStore("")
	if err := store.Keep("d65f03c0", "one", InstructionTrace{}); err != nil {
		t.Fatalf("Keep() error = %v", err)
	}
	if _, found := store.Lookup("d65f03c0", "one"); found {
		t.Fatal("Lookup() found a trace in a disabled store")
	}
}
