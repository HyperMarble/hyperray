//go:build isla_integration && arm64_acceptance

// These tests protect the exact ARM witness parser from radix and alias bugs.
// They must not invoke a solver or use a substring assertion.
package isla_test

import "testing"

func TestARM64RadixValueUsesDeclaredBase(t *testing.T) {
	cases := map[string]uint64{"10": 10, "#x10": 16, "#b10": 2, "0x10": 16, "0b10": 2}
	for input, want := range cases {
		value, err := arm64RadixValue(input)
		if err != nil || value != want {
			t.Errorf("arm64RadixValue(%q) = %d, %v; want %d", input, value, err, want)
		}
	}
}

func TestARM64RadixValueRejectsInvalidUnsigned(t *testing.T) {
	for _, input := range []string{"-1", "#x", "0x10000000000000000"} {
		if _, err := arm64RadixValue(input); err == nil {
			t.Errorf("arm64RadixValue(%q) accepted invalid unsigned value", input)
		}
	}
}

func TestARM64WitnessValueRejectsAmbiguousAliases(t *testing.T) {
	assignments := map[string]string{"0:R0": "#x4", "0:X0": "#x4"}
	if _, err := arm64WitnessValue(assignments, []string{"0:R0", "0:X0"}); err == nil {
		t.Fatal("arm64WitnessValue accepted duplicate register aliases")
	}
}

func TestARM64AssignmentsRejectsDuplicateKeys(t *testing.T) {
	if _, err := arm64Assignments("0:R0=#x4;0:R0=#x4;"); err == nil {
		t.Fatal("arm64Assignments accepted duplicate keys")
	}
}
