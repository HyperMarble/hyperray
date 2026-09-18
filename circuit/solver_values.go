// Solver-value parsing accepts only the atomic values requested by the miter.
// It never guesses a missing assignment or silently accepts an extra value.
package circuit

import "strings"

func solverTokens(output string) []string {
	replacer := strings.NewReplacer("(", " ( ", ")", " ) ")
	return strings.Fields(replacer.Replace(output))
}

func solverValues(tokens []string, expected map[string]struct{}) (map[string]string, error) {
	if len(tokens) < 2 || tokens[0] != "(" || tokens[len(tokens)-1] != ")" {
		return nil, engineError("malformed_solver_model", "get-value")
	}
	values := make(map[string]string, len(expected))
	for position := 1; position < len(tokens)-1; position += 4 {
		if position+3 >= len(tokens)-1 {
			return nil, engineError("malformed_solver_model", "get-value")
		}
		if tokens[position] != "(" || tokens[position+3] != ")" {
			return nil, engineError("malformed_solver_model", "get-value")
		}
		symbol := tokens[position+1]
		if _, exists := expected[symbol]; !exists {
			return nil, engineError("unexpected_solver_value", symbol)
		}
		if _, exists := values[symbol]; exists {
			return nil, engineError("duplicate_solver_value", symbol)
		}
		values[symbol] = tokens[position+2]
	}
	if len(values) != len(expected) {
		return nil, engineError("incomplete_solver_model", "get-value")
	}
	return values, nil
}

func expectedSymbols(miter Miter) map[string]struct{} {
	expected := make(map[string]struct{}, len(miter.assignments)+2*len(miter.outputs))
	for _, assignment := range miter.assignments {
		expected[assignment.symbol] = struct{}{}
	}
	for _, output := range miter.outputs {
		expected[output.referenceSymbol] = struct{}{}
		expected[output.candidateSymbol] = struct{}{}
	}
	return expected
}
