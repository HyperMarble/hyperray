// ARM64 observation range tests use uint64 extent arithmetic.
package isla

import "testing"

func TestObservationRangeAllowsHighUint64Addresses(t *testing.T) {
	end, ok := observationRange(1<<48, 2)
	if !ok || end != 1<<48+2 {
		t.Fatalf("observationRange() = (%#x, %v), want high address extent", end, ok)
	}
}

func TestObservationRangeRejectsUint64Overflow(t *testing.T) {
	if _, ok := observationRange(^uint64(0), 1); ok {
		t.Fatal("observationRange() accepted uint64 overflow")
	}
}
