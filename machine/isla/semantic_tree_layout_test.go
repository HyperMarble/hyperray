// Layout fixtures preserve event identity across whitespace and quoted data.
package isla

import "testing"

func TestSemanticTreeLayoutPreservesEvents(t *testing.T) {
	cases := []string{
		`(trace (read-reg |PC| nil #x1000)(instr #x0013))`,
		"(trace\n(read-reg\n|PC|\tnil\n#x1000)\n(instr\n#x0013)\n)",
		`(trace (read-reg |PC| nil #x1000)
 (cases "source (path)"
 (trace (declare-const |v(with)parentheses| "literal \" (text)")
 ; a comment (with unmatched parentheses
 (instr #x0013))))`,
		`(trace (read-reg |PC| nil #x1000)
 (cases "source" (trace (cases "nested" (trace (instr #x0013))))))`,
	}
	for index, tree := range cases {
		instructions, _, count, err := semanticInstructionEvents(tree)
		if err != nil || count != 1 || len(instructions) != 1 {
			t.Errorf("case %d = %v, %d, %v", index, instructions, count, err)
		}
	}
}

func TestSemanticTreeRepeatedEventsKeepExactCounts(t *testing.T) {
	tree := `(trace (read-reg |PC| nil #x1000)
 (cases "source" (trace (instr #x0013)) (trace (instr #x0013))))`
	instructions, encodings, count, err := semanticInstructionEvents(tree)
	if err != nil || count != 2 || len(instructions) != 1 || len(encodings) != 1 {
		t.Errorf("events = %v, %v, %d, %v", instructions, encodings, count, err)
	}
}
