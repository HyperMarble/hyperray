// SMT definitions keep reference and candidate outputs independently named.
// They never compare expressions before both meanings are visible.
package circuit

import "strconv"

func declaration(symbol string, width uint16) string {
	return "(declare-fun " + symbol + " () (_ BitVec " + strconv.Itoa(int(width)) + "))"
}

func definitionLines(
	relation StepRelation,
	outputs []outputQuery,
	symbols map[string]string,
	reference bool,
) ([]string, error) {
	lines := make([]string, 0, len(outputs))
	for position, nextState := range relation.nextStates {
		value, err := bitVectorSMT(nextState.value.node, symbols)
		if err != nil {
			return nil, err
		}
		symbol := selectedSymbol(outputs[position], reference)
		lines = append(lines, bitVectorDefinition(symbol, value, relation.states[position].width))
	}
	offset := len(relation.nextStates)
	for position, observation := range relation.observations {
		value, err := booleanSMT(observation.value, symbols)
		if err != nil {
			return nil, err
		}
		symbol := selectedSymbol(outputs[offset+position], reference)
		lines = append(lines, booleanDefinition(symbol, value))
	}
	return lines, nil
}

func selectedSymbol(output outputQuery, reference bool) string {
	if reference {
		return output.referenceSymbol
	}
	return output.candidateSymbol
}

func bitVectorDefinition(symbol string, value string, width uint16) string {
	return "(define-fun " + symbol + " () (_ BitVec " + strconv.Itoa(int(width)) + ") " + value + ")"
}

func booleanDefinition(symbol string, value string) string {
	return "(define-fun " + symbol + " () Bool " + value + ")"
}
