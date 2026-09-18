// The final SMT clauses select a stable difference and concrete assignment.
// They never request a positive proof record from a tool that cannot supply one.
package circuit

import "strings"

func differenceLine(outputs []outputQuery) string {
	differences := make([]string, 0, len(outputs))
	for _, output := range outputs {
		difference := "(distinct " + output.referenceSymbol + " " + output.candidateSymbol + ")"
		differences = append(differences, difference)
	}
	if len(differences) == 1 {
		return "(assert " + differences[0] + ")"
	}
	return "(assert (or " + strings.Join(differences, " ") + "))"
}

func minimizationLines(assignments []assignmentQuery) []string {
	lines := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		lines = append(lines, "(minimize (bv2int "+assignment.symbol+"))")
	}
	return lines
}

func valueQueryLine(assignments []assignmentQuery, outputs []outputQuery) string {
	symbols := make([]string, 0, len(assignments)+2*len(outputs))
	for _, assignment := range assignments {
		symbols = append(symbols, assignment.symbol)
	}
	for _, output := range outputs {
		symbols = append(symbols, output.referenceSymbol, output.candidateSymbol)
	}
	return "(get-value (" + strings.Join(symbols, " ") + "))"
}

func witnessInput(miter Miter) string {
	return miter.smt2 + valueQueryLine(miter.assignments, miter.outputs) + "\n"
}
