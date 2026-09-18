// Invalid tree fixtures cannot silently omit branches or fabricate a PC.
package isla

import "testing"

func TestSemanticTreeRejectsMalformedTrees(t *testing.T) {
	cases := []string{
		`(trace (cases "source"))`,
		`(trace (cases source (trace (cycle))))`,
		`(trace (cases "source""extra" (trace (cycle))))`,
		`(trace (cases "source" (trace (cycle))) (cycle))`,
		`(trace (cases "source" (instr #x0013)))`,
		`(trace (trace (cycle)))`,
		`(trace (cases "source" (trace)))`,
		`(trace (cycle)) (trace (cycle))`,
		`(trace (read-reg |PC| nil #x1000) (instr #x0013) (instr #x0013))`,
		`(trace (cases "source" (trace (read-reg |PC| nil #x1000)) (trace (instr #x0013))))`,
		`(trace (cycle) garbage)`,
		`(trace (cycle) (cases "source" (trace (cycle)))`,
		`(trace (read-reg |PC| nil v1) (instr #x0013))`,
		`(trace (read-reg |PC| nil #x1000) (instr #x0013 extra))`,
		`(trace (read-reg |PC| nil #x1000) (instr #xzz))`,
		`(trace (event "unfinished))`,
		`(trace (event |unfinished))`,
		`(trace (cycle) extra))`,
	}
	for index, tree := range cases {
		if _, _, _, err := semanticInstructionEvents(tree); err == nil {
			t.Errorf("accepted malformed case %d: %s", index, tree)
		}
	}
}
