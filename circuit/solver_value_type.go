// Solver-value validation binds each atom to its declared finite sort.
// It never reports malformed text as a concrete bit-vector assignment.
package circuit

import "strings"

func solverValueTypesError(miter Miter, values map[string]string) error {
	for _, assignment := range miter.assignments {
		if err := bitVectorValueError(values[assignment.symbol], assignment.width); err != nil {
			return engineError("invalid_solver_value", assignment.symbol, values[assignment.symbol])
		}
	}
	for _, output := range miter.outputs {
		if err := outputPairValueError(output, values); err != nil {
			return err
		}
	}
	return nil
}

func outputPairValueError(output outputQuery, values map[string]string) error {
	if err := outputValueError(output, values[output.referenceSymbol]); err != nil {
		return engineError("invalid_solver_value", output.referenceSymbol, values[output.referenceSymbol])
	}
	if err := outputValueError(output, values[output.candidateSymbol]); err != nil {
		return engineError("invalid_solver_value", output.candidateSymbol, values[output.candidateSymbol])
	}
	return nil
}

func outputValueError(output outputQuery, value string) error {
	switch output.kind {
	case NextStateOutput:
		return bitVectorValueError(value, output.width)
	case ObservationOutput:
		if value != "true" && value != "false" {
			return engineError("invalid_boolean_value", value)
		}
		return nil
	default:
		return engineError("unknown_output_kind", string(output.kind))
	}
}

func bitVectorValueError(value string, width uint16) error {
	if width == 0 {
		return engineError("invalid_bit_vector_value", value)
	}
	prefix := "#b"
	digitCount := int(width)
	alphabet := "01"
	if width%4 == 0 {
		prefix = "#x"
		digitCount = int(width / 4)
		alphabet = "0123456789abcdefABCDEF"
	}
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+digitCount {
		return engineError("invalid_bit_vector_value", value)
	}
	for _, digit := range value[len(prefix):] {
		if !strings.ContainsRune(alphabet, digit) {
			return engineError("invalid_bit_vector_value", value)
		}
	}
	return nil
}
