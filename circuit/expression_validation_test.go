// Expression tests cover all public typed-construction failures.
// They never inject textual SMT or bypass expression constructors.
package circuit_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestRelationValidationRejectsUnknownBitVectors(t *testing.T) {
	valid := circuit.Read(mustVariable(t, "value", 8))
	_, err := circuit.Subtract(circuit.BitVectorExpression{}, valid)
	requireEngineError(t, err, "unknown_bit_vector_semantics", "expression")
	_, err = circuit.Subtract(valid, circuit.BitVectorExpression{})
	requireEngineError(t, err, "unknown_bit_vector_semantics", "expression")
	_, err = circuit.NewNextState("value", circuit.Read(circuit.Variable{}))
	requireEngineError(t, err, "empty_identifier", "expression.variable")
}

func TestRelationValidationRejectsBitVectorWidthMismatch(t *testing.T) {
	left := circuit.Read(mustVariable(t, "left", 8))
	right := circuit.Read(mustVariable(t, "right", 16))
	_, err := circuit.Subtract(left, right)
	requireEngineError(t, err, "width_mismatch", "subtract")
}

func TestRelationValidationRejectsComparisonOperands(t *testing.T) {
	valid := circuit.Read(mustVariable(t, "value", 8))
	wide := circuit.Read(mustVariable(t, "wide", 16))
	_, err := circuit.UnsignedLessThan(circuit.BitVectorExpression{}, valid)
	requireEngineError(t, err, "unknown_bit_vector_semantics", "expression")
	_, err = circuit.UnsignedLessThan(valid, circuit.BitVectorExpression{})
	requireEngineError(t, err, "unknown_bit_vector_semantics", "expression")
	_, err = circuit.UnsignedLessThan(valid, wide)
	requireEngineError(t, err, "width_mismatch", "comparison")
}

func TestRelationValidationRejectsInvalidOutputs(t *testing.T) {
	value := circuit.Read(mustVariable(t, "value", 8))
	_, err := circuit.NewNextState("bad name", value)
	requireEngineError(t, err, "invalid_identifier", "next_state.name", "bad name")
	_, err = circuit.NewNextState("value", circuit.BitVectorExpression{})
	requireEngineError(t, err, "unknown_bit_vector_semantics", "expression")
	_, err = circuit.NewObservation("bad name", circuit.BooleanExpression{})
	requireEngineError(t, err, "invalid_identifier", "observation.name", "bad name")
	_, err = circuit.NewObservation("flag", circuit.BooleanExpression{})
	requireEngineError(t, err, "unknown_boolean_semantics", "expression")
}
