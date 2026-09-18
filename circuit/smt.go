// SMT generation creates one deterministic miter over shared step inputs.
// It never asks the proposal engine for a proof or treats a result as one.
package circuit

import "strings"

func writeMiter(
	reference StepRelation,
	candidate StepRelation,
) (string, []assignmentQuery, []outputQuery, error) {
	assignments := relationAssignments(reference)
	outputs := relationOutputs(reference)
	symbols := variableSymbols(assignments)
	lines := []string{"(set-option :produce-models true)", "(set-option :opt.priority lex)"}
	lines = append(lines, declarationLines(reference, assignments)...)
	referenceLines, err := definitionLines(reference, outputs, symbols, true)
	if err != nil {
		return "", nil, nil, err
	}
	candidateLines, err := definitionLines(candidate, outputs, symbols, false)
	if err != nil {
		return "", nil, nil, err
	}
	lines = append(lines, referenceLines...)
	lines = append(lines, candidateLines...)
	lines = append(lines, differenceLine(outputs))
	lines = append(lines, minimizationLines(assignments)...)
	lines = append(lines, "(check-sat)")
	return strings.Join(lines, "\n") + "\n", assignments, outputs, nil
}

func variableSymbols(assignments []assignmentQuery) map[string]string {
	symbols := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		symbols[assignment.name] = assignment.symbol
	}
	return symbols
}

func declarationLines(relation StepRelation, assignments []assignmentQuery) []string {
	variables := append(append([]Variable(nil), relation.states...), relation.inputs...)
	lines := make([]string, 0, len(variables))
	for position, variable := range variables {
		lines = append(lines, declaration(assignments[position].symbol, variable.width))
	}
	return lines
}
