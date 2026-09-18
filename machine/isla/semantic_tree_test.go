// Tree tests preserve writer structure and never infer instruction meaning.
package isla

import "testing"

func TestSemanticTreeRestoresParentPC(t *testing.T) {
	tree := `(trace
 (read-reg |PC| nil #x1000)
 (cases "source"
  (trace (read-reg |PC| nil #x2000) (instr #x0013))
  (trace (instr #x459d))))`
	instructions, _, count, err := semanticInstructionEvents(tree)
	if err != nil || count != 2 {
		t.Fatalf("events = %v, %d, %v", instructions, count, err)
	}
	for _, instruction := range []SemanticInstruction{{0x2000, "00000013"}, {0x1000, "0000459d"}} {
		if _, found := instructions[instruction]; !found {
			t.Errorf("missing instruction %v in %v", instruction, instructions)
		}
	}
}

func TestSemanticTreeIgnoresNestedEventData(t *testing.T) {
	tree := `(trace (read-reg |PC| nil #x1000)
 (abstract-call |name| (instr #x459d) nil)
 (instr
 #x0013))`
	instructions, _, count, err := semanticInstructionEvents(tree)
	if err != nil || count != 1 || len(instructions) != 1 {
		t.Fatalf("events = %v, %d, %v", instructions, count, err)
	}
	if _, found := instructions[SemanticInstruction{0x1000, "00000013"}]; !found {
		t.Errorf("wrong instruction set %v", instructions)
	}
}

func TestSemanticTreeTracksEveryFirstPCAcrossSiblingOrders(t *testing.T) {
	cases := []string{
		`(trace (cases "source" (trace (read-reg |PC| nil #x1000) (instr #x0013)) (trace (read-reg |PC| nil #x2000) (instr #x0013))))`,
		`(trace (cases "source" (trace (read-reg |PC| nil #x2000) (instr #x0013)) (trace (read-reg |PC| nil #x1000) (instr #x0013))))`,
	}
	for index, tree := range cases {
		inventory, err := semanticInstructionInventory(tree)
		if err != nil || !sameAddresses(inventory.entryAddresses, 0x1000, 0x2000) {
			t.Errorf("case %d first PCs = %#v, error = %v", index, inventory.entryAddresses, err)
		}
	}
}

func TestSemanticTreeAcceptsEqualAndSharedFirstPCs(t *testing.T) {
	cases := []string{
		`(trace (cases "source" (trace (read-reg |PC| nil #x1000) (instr #x0013)) (trace (read-reg |PC| nil #x1000) (instr #x0013))))`,
		`(trace (read-reg |PC| nil #x1000) (instr #x0013) (cases "source" (trace (read-reg |PC| nil #x1004) (instr #x0013)) (trace (read-reg |PC| nil #x1004) (instr #x0013))))`,
	}
	for index, tree := range cases {
		inventory, err := semanticInstructionInventory(tree)
		if err != nil || !sameAddresses(inventory.entryAddresses, 0x1000) {
			t.Errorf("case %d first PCs = %#v, error = %v", index, inventory.entryAddresses, err)
		}
	}
}

func sameAddresses(values map[uint64]struct{}, wanted ...uint64) bool {
	if len(values) != len(wanted) {
		return false
	}
	for _, address := range wanted {
		if _, found := values[address]; !found {
			return false
		}
	}
	return true
}
