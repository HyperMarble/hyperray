// Count tests reject missing, malformed, and inconsistent witness totals.
// They must accept both valid outcome classes.
package isla

import "testing"

func TestWitnessCounts(t *testing.T) {
	positive, negative, err := witnessCounts([]string{"Witnesses", "Positive: 2 Negative: 3"})
	if err != nil {
		t.Fatalf("witnessCounts() error = %v", err)
	}
	if positive != 2 || negative != 3 {
		t.Errorf("witnessCounts() = %d, %d", positive, negative)
	}
}

func TestWitnessCountErrors(t *testing.T) {
	cases := [][]string{
		{"none"},
		{"Positive: bad Negative: 0"},
		{"Positive: 0 Negative: bad"},
	}
	for index := range cases {
		positive, negative, err := witnessCounts(cases[index])
		if err == nil {
			t.Errorf("witnessCounts() = %d, %d, nil", positive, negative)
		}
		assertInternalCode(t, err, ProtocolError)
	}
}

func TestConsistentResults(t *testing.T) {
	if err := consistentResult("Allowed", 1); err != nil {
		t.Errorf("consistentResult(Allowed) error = %v", err)
	}
	if err := consistentResult("Forbidden", 0); err != nil {
		t.Errorf("consistentResult(Forbidden) error = %v", err)
	}
	assertInternalCode(t, consistentResult("Allowed", 0), ProtocolError)
	assertInternalCode(t, consistentResult("Forbidden", 1), ProtocolError)
}
