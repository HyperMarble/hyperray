// These tests cover sort validation for opaque solver-query records.
// They never permit malformed atoms in a public concrete difference.
package circuit

import "testing"

func TestDefensiveBitVectorValues(t *testing.T) {
	values := []struct {
		value string
		width uint16
	}{{"#x0", 8}, {"#b00000000", 8}, {"#x0g", 8}, {"#x", 0}}
	for _, item := range values {
		err := bitVectorValueError(item.value, item.width)
		requireInternalCode(t, err, "invalid_bit_vector_value")
	}
	if err := bitVectorValueError("#b101", 3); err != nil {
		t.Errorf("bitVectorValueError() error = %v", err)
	}
}

func TestDefensiveOutputValues(t *testing.T) {
	booleanOutput := outputQuery{kind: ObservationOutput}
	err := outputValueError(booleanOutput, "unknown")
	requireInternalCode(t, err, "invalid_boolean_value")
	err = outputValueError(outputQuery{kind: "unsupported"}, "false")
	requireInternalCode(t, err, "unknown_output_kind")
}

func TestDefensiveOutputPairValues(t *testing.T) {
	output := outputQuery{
		kind: NextStateOutput, width: 8,
		referenceSymbol: "reference", candidateSymbol: "candidate",
	}
	values := map[string]string{"reference": "#x00", "candidate": "bad"}
	err := outputPairValueError(output, values)
	requireInternalCode(t, err, "invalid_solver_value")
	values = map[string]string{"reference": "bad", "candidate": "#x00"}
	err = outputPairValueError(output, values)
	requireInternalCode(t, err, "invalid_solver_value")
	miter := Miter{outputs: []outputQuery{output}}
	err = solverValueTypesError(miter, values)
	requireInternalCode(t, err, "invalid_solver_value")
}
