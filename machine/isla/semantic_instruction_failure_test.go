// Semantic PC tests reject symbolic, malformed, and missing instruction locations.
package isla

import "testing"

func TestSemanticInstructionRejectsInvalidProgramCounters(t *testing.T) {
	cases := []string{
		"(read-reg |PC| nil symbolic)",
		"(read-reg |PC| nil #xzz)",
	}
	for index := range cases {
		tree := "(trace\n  " + cases[index] + "\n  (instr #x00000013)\n)"
		if _, _, _, err := semanticInstructionEvents(tree); err == nil {
			t.Errorf("semanticInstructionEvents() accepted case %d", index)
		}
	}
	if _, _, _, err := semanticInstructionEvents("(trace\n  (instr #x00000013)\n)"); err == nil {
		t.Error("semanticInstructionEvents() accepted a missing PC")
	}
}

func TestSemanticInstructionAcceptsARMProgramCounter(t *testing.T) {
	tree := "(trace\n  (read-reg |_PC| nil #x00000001000002e8)\n  (instr #x91000400)\n)"
	instructions, encodings, count, err := semanticInstructionEvents(tree)
	if err != nil {
		t.Fatalf("semanticInstructionEvents() error = %v", err)
	}
	if count != 1 || len(instructions) != 1 || len(encodings) != 1 {
		t.Fatalf("semantic instruction inventory = %d/%d/%d, want 1/1/1", count, len(instructions), len(encodings))
	}
}
