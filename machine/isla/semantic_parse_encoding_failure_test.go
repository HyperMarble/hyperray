// Encoding failure tests reject malformed records and unequal coverage sets.
// They do not encode any instruction meaning in the parser.
package isla

import "testing"

func TestSemanticParserRejectsMalformedInstructionEvents(t *testing.T) {
	cases := []string{
		"(instr #x00300293 extra)",
		"(instr #xzz)",
	}
	for index := range cases {
		tree := "Thread 0:\n(trace\n  (read-reg |PC| nil #x1000)\n  " + cases[index] + "\n)"
		requireSemanticError(t, semanticTestOutput(tree, "True", ""), ProtocolError)
	}
}

func TestSemanticParserRejectsMalformedFootprints(t *testing.T) {
	tree := "Thread 0:\n(trace\n  (read-reg |PC| nil #x1000)\n  (instr #x00300293)\n)"
	cases := []string{
		"opcode 00300293",
		"opcode zz, Footprint:",
		"opcode 00300293, Footprint:\nopcode 00300293, Footprint:",
	}
	for index := range cases {
		requireSemanticError(t, semanticTestOutput(tree, "True", cases[index]), ProtocolError)
	}
}

func TestSemanticParserRejectsDifferentEqualSizeSets(t *testing.T) {
	tree := "Thread 0:\n(trace\n  (read-reg |PC| nil #x1000)\n  (instr #x00300293)\n)"
	output := semanticTestOutput(tree, "True", "opcode 00000013, Footprint:")
	requireSemanticError(t, output, CoverageMismatch)
}
