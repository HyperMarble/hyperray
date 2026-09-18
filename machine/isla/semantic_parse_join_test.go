// Parser-to-join tests isolate branch-local entry ownership.
// Every instruction encoding belongs to the loaded inventory.
package isla

import "testing"

func TestSemanticParserJoinRejectsPreInstructionForksInEitherSiblingOrder(t *testing.T) {
	program := internalTwoThreadProgram()
	entry := `(trace (read-reg |PC| nil #x1000) (instr #x00000013))`
	other := `(trace (read-reg |PC| nil #x2000) (instr #x00000093))`
	baseline := parsedForkReport(t, "", entry, entry)
	if err := matchProgramSemantics(program, baseline); err != nil {
		t.Fatalf("equal-entry baseline rejected: %v", err)
	}
	orders := [][2]string{{entry, other}, {other, entry}}
	for index, order := range orders {
		report := parsedForkReport(t, "", order[0], order[1])
		failure, ok := matchError(matchProgramSemantics(program, report))
		if !ok || failure.Code != CoverageMismatch || failure.Subject != "executed instruction inventory" || failure.Detail != "thread entry differs" {
			t.Errorf("sibling order %d error = %#v, want thread entry mismatch", index, failure)
		}
	}
}

func TestSemanticParserJoinAcceptsSharedInstructionPrefix(t *testing.T) {
	program := internalTwoThreadProgram()
	prefix := `(read-reg |PC| nil #x1000) (instr #x00000013)`
	child := `(trace (read-reg |PC| nil #x2000) (instr #x00000093))`
	report := parsedForkReport(t, prefix, child, child)
	if err := matchProgramSemantics(program, report); err != nil {
		t.Fatalf("shared-prefix report rejected: %v", err)
	}
}

func parsedForkReport(t *testing.T, prefix, first, second string) SemanticReport {
	t.Helper()
	threads := "Thread 0:\n(trace " + prefix + ` (cases "source" ` + first + " " + second + "))\n" +
		"Thread 1:\n(trace (read-reg |PC| nil #x2000) (instr #x00000093))"
	summary, err := parseSemanticOutput(semanticTestOutput(threads, "True", "opcode 00000013, Footprint:\nopcode 00000093, Footprint:"))
	if err != nil {
		t.Fatalf("fork parse error = %v", err)
	}
	return SemanticReport{Complete: true, ThreadCount: summary.threadCount, Instructions: summary.instructions, Threads: summary.threads}
}
