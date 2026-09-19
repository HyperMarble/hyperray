// Split and rejoin tests keep each trace with the instruction it came from.
// A trace must never move to a neighbour when some are already stored.
package isla

import "testing"

func namedTrace(encoding string) InstructionTrace {
	return InstructionTrace{Encoding: encoding, TraceCount: 1}
}

func TestPlaceTracedFillsOnlyTheHoles(t *testing.T) {
	traces := []InstructionTrace{namedTrace("aaaa"), {}, namedTrace("cccc"), {}}
	traced := []InstructionTrace{namedTrace("bbbb"), namedTrace("dddd")}
	placed := placeTraced(traces, traced)
	want := []string{"aaaa", "bbbb", "cccc", "dddd"}
	for index := range want {
		if placed[index].Encoding != want[index] {
			t.Errorf("placeTraced()[%d] = %q, want %q", index, placed[index].Encoding, want[index])
		}
	}
}

func TestPlaceTracedKeepsOrderWhenNothingIsStored(t *testing.T) {
	traces := []InstructionTrace{{}, {}, {}}
	traced := []InstructionTrace{namedTrace("one"), namedTrace("two"), namedTrace("three")}
	placed := placeTraced(traces, traced)
	for index, want := range []string{"one", "two", "three"} {
		if placed[index].Encoding != want {
			t.Errorf("placeTraced()[%d] = %q, want %q", index, placed[index].Encoding, want)
		}
	}
}

func TestTraceWorkerLimitNeverExceedsTheWork(t *testing.T) {
	if limit := traceWorkerLimit(1); limit != 1 {
		t.Errorf("traceWorkerLimit(1) = %d, want 1", limit)
	}
	if limit := traceWorkerLimit(0); limit != 0 {
		t.Errorf("traceWorkerLimit(0) = %d, want 0", limit)
	}
}
