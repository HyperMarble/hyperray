// Successful parser tests cover multiple threads and stable encoding sets.
// Repeated instruction events remain events but not duplicate set members.
package isla

import "testing"

func TestSemanticParserRecordsAllThreadsAndInstructions(t *testing.T) {
	threads := `
Thread 0:
(trace
  (read-reg |PC| nil #x0000000000001000)
  (instr #x00000013)
  (read-reg |PC| nil #x0000000000001004)
  (instr #x00300293)
)
Thread 1:
(trace
  (read-reg |PC| nil #x0000000000002000)
  (instr #x00300293)
)`
	footprints := "opcode 00300293, Footprint:\nopcode 00000013, Footprint:"
	summary, err := parseSemanticOutput(semanticTestOutput(threads, "Not(Eq(x5, 3))", footprints))
	if err != nil {
		t.Fatalf("parseSemanticOutput() error = %v", err)
	}
	if summary.threadCount != 2 || summary.traceCount != 2 || summary.instructionEventCount != 3 {
		t.Errorf("summary = %#v", summary)
	}
	if len(summary.threads) != 2 || summary.threads[0].EntryAddress != 0x1000 || summary.threads[1].EntryAddress != 0x2000 ||
		len(summary.threads[0].EntryAddresses) != 1 || summary.threads[0].EntryAddresses[0] != 0x1000 ||
		len(summary.threads[1].EntryAddresses) != 1 || summary.threads[1].EntryAddresses[0] != 0x2000 {
		t.Errorf("per-thread entries = %#v", summary.threads)
	}
	want := []string{"00000013", "00300293"}
	for index := range want {
		if summary.instructionEncodings[index] != want[index] || summary.footprintEncodings[index] != want[index] {
			t.Errorf("encodings = %v footprints = %v", summary.instructionEncodings, summary.footprintEncodings)
		}
	}
}

func TestSemanticParserAcceptsNoExecutedInstruction(t *testing.T) {
	output := semanticTestOutput("Thread 0:\n(trace\n  (cycle)\n)", "True", "")
	summary, err := parseSemanticOutput(output)
	if err != nil || len(summary.instructionEncodings) != 0 {
		t.Errorf("parseSemanticOutput() = %#v, %v", summary, err)
	}
}

func TestSemanticParserMatchesCompressedEncodingWidths(t *testing.T) {
	tree := "Thread 0:\n(trace\n  (read-reg |PC| nil #x1000)\n  (instr #x459d)\n)"
	footprint := "opcode 0000459d, Footprint:"
	summary, err := parseSemanticOutput(semanticTestOutput(tree, "True", footprint))
	if err != nil {
		t.Fatalf("parseSemanticOutput() error = %v", err)
	}
	if len(summary.instructionEncodings) != 1 || summary.instructionEncodings[0] != "0000459d" {
		t.Errorf("instruction encodings = %v", summary.instructionEncodings)
	}
}
