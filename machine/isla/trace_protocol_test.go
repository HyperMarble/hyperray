// Trace-protocol tests reject nontrace and truncated tool output.
package isla

import "testing"

func TestCountTraceBlocks(t *testing.T) {
	count, err := countTraceBlocks("(trace\n  (event one)\n)\n(trace\n  (event two)\n)\n")
	if err != nil || count != 2 {
		t.Errorf("countTraceBlocks() = %d, %v", count, err)
	}
}

func TestCountTraceBlocksRejectsMalformedOutput(t *testing.T) {
	values := []string{"message", "(trace\n  (event)"}
	for index := range values {
		count, err := countTraceBlocks(values[index])
		if err == nil {
			t.Errorf("case %d count = %d, error = nil", index, count)
		}
	}
}
