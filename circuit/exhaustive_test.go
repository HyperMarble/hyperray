// Exhaustive tests measure every balance and cost pair in each 8-bit fixture.
// They never sample the finite input domain.
package circuit_test

import "testing"

func TestBalanceFixturesExhaustive(t *testing.T) {
	for _, fixture := range loadBalanceFixtures(t) {
		if fixture.Width != 8 {
			t.Errorf("fixture width = %d, want 8", fixture.Width)
			continue
		}
		differentPairs, measuredPairs := measureFixture(t, fixture)
		if measuredPairs != 65536 {
			t.Errorf("measured pairs = %d, want 65536", measuredPairs)
		}
		if differentPairs != fixture.DifferentPairs {
			t.Errorf("different pairs = %d, want %d", differentPairs, fixture.DifferentPairs)
		}
	}
}

func measureFixture(t *testing.T, fixture balanceFixture) (uint64, uint64) {
	t.Helper()
	var differentPairs uint64
	var measuredPairs uint64
	for balance := uint16(0); balance < 256; balance++ {
		for cost := uint16(0); cost < 256; cost++ {
			reference := compareValues(t, fixture.ReferenceComparison, balance, cost)
			candidate := compareValues(t, fixture.CandidateComparison, balance, cost)
			differentPairs += differenceCount(reference, candidate)
			measuredPairs++
		}
	}
	return differentPairs, measuredPairs
}

func compareValues(t *testing.T, operation string, left uint16, right uint16) bool {
	t.Helper()
	switch operation {
	case "unsigned_less_than":
		return left < right
	case "unsigned_less_or_equal":
		return left <= right
	default:
		t.Fatalf("unknown comparison %q", operation)
		return false
	}
}

func differenceCount(reference bool, candidate bool) uint64 {
	if reference != candidate {
		return 1
	}
	return 0
}
