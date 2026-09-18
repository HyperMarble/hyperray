// Section failure tests reject missing, repeated, empty, and reordered data.
// No incomplete semantic document can produce a summary.
package isla

import "testing"

func TestSemanticParserRejectsInvalidSections(t *testing.T) {
	tree := "Thread 0:\n(trace\n  (cycle)\n)"
	cases := []string{
		tree + "\nMemory:test",
		tree + "\nFinal Assertion:\nTrue\nFinal Assertion:\nTrue\nMemory:test",
		tree + "\nFinal Assertion:\nTrue",
		tree + "\nFinal Assertion:\nTrue\nMemory:test\nMemory:again",
		tree + "\nMemory:test\nFinal Assertion:\nTrue",
		tree + "\nFinal Assertion:\n\nMemory:test",
		"Final Assertion:\nTrue\nMemory:test",
	}
	for index := range cases {
		requireSemanticError(t, cases[index], ProtocolError)
	}
}

func TestSemanticParserRejectsInvalidThreadHeaders(t *testing.T) {
	cases := []string{
		"text\n(trace\n  (cycle)\n)",
		"Thread 1:\n(trace\n  (cycle)\n)",
		"Thread bad:\n(trace\n  (cycle)\n)",
		"Thread 0:\n",
	}
	for index := range cases {
		output := semanticTestOutput(cases[index], "True", "")
		requireSemanticError(t, output, ProtocolError)
	}
}
