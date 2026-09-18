// Pure decisions test measurement arithmetic and explicit stop reasons.
// These tests must not substitute for real process observations.
package execution

import (
	"context"
	"math"
	"testing"
)

func TestCombinedPeak(t *testing.T) {
	for _, pair := range [][2]int64{{0, 1}, {1, 0}, {-1, 1}, {math.MaxInt64, 1}} {
		if _, err := combinedPeak(pair[0], pair[1]); err == nil {
			t.Errorf("invalid measurements accepted: %v", pair)
		}
	}
	value, err := combinedPeak(11, 13)
	if err != nil || value != 24 {
		t.Fatalf("sum = %d, error = %v", value, err)
	}
}

func TestOutcome(t *testing.T) {
	cases := []struct {
		cause    error
		exceeded bool
		peak     int64
		want     Status
	}{
		{nil, false, 99, Exited}, {nil, false, 100, MemoryLimit},
		{context.Canceled, false, 10, Canceled},
		{context.DeadlineExceeded, false, 10, TimedOut},
		{context.Canceled, true, 10, OutputLimit},
	}
	for _, item := range cases {
		if got := outcome(item.cause, item.exceeded, item.peak, 100); got != item.want {
			t.Errorf("outcome = %s, want %s", got, item.want)
		}
	}
}

func TestRetainedBytes(t *testing.T) {
	for _, item := range [][3]int{{0, 9, 0}, {8, 9, 8}, {9, 9, 9}, {10, 9, 9}} {
		if got := retainedBytes(item[0], item[1]); got != item[2] {
			t.Errorf("retained bytes = %d, want %d", got, item[2])
		}
	}
}
