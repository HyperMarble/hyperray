// Labeled mixed outcomes must select an allowed concrete row in either order.
// Missing labels, state counts, or values must never become a witness.
package isla

import "testing"

func TestLabeledCounterexampleRows(t *testing.T) {
	cases := [][]string{
		{"States 2", "forbidden ???;", "allowed 0:x10=0x40;"},
		{"States 2", "allowed 0:x10=0x40;", "forbidden ???;"},
	}
	for _, lines := range cases {
		state, err := resultState(lines, 1, 1)
		if err != nil || state != "0:x10=0x40;" {
			t.Errorf("state = %q, %v", state, err)
		}
	}
}

func TestLabeledCounterexampleRejectsAmbiguity(t *testing.T) {
	cases := [][]string{
		{"States 2", "???;", "0:x10=64;"},
		{"States 2", "forbidden ???;", "allowed 0:x10=v2656;"},
		{"States 2", "forbidden 0:x10=0;", "allowed 0:x10=64;"},
		{"States 2", "allowed 0:x10=17;", "allowed 0:x10=64;"},
		{"States 1", "allowed 0:x10=64;"},
		{"States 2", "allowed 0:x10=64;"},
		{"States 2", "allowed 0:x10=64;", "forbidden ???;", "States 2"},
		{"States bad", "allowed 0:x10=64;"},
		{"States +2", "allowed 0:x10=64;", "forbidden ???;"},
		{"States 02", "allowed 0:x10=64;", "forbidden ???;"},
		{"States 2 ", "allowed 0:x10=64;", "forbidden ???;"},
	}
	for index, lines := range cases {
		if state, err := resultState(lines, 1, 1); err == nil || state != "" {
			t.Errorf("accepted case %d: %q, %v", index, state, err)
		}
	}
	if _, err := candidateRows(nil, ^uint64(0), 1); err == nil {
		t.Error("accepted overflowing candidate counts")
	}
}
