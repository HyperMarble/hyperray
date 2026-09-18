// These tests cover defensive SMT errors behind validated relations.
// They never accept an unknown operation as an uninterpreted solver term.
package circuit

import "testing"

func TestDefensiveBitVectorSMTErrors(t *testing.T) {
	symbols := map[string]string{"value": "|value|"}
	_, err := bitVectorSMT(nil, symbols)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	_, err = bitVectorSMT(&bitVectorNode{operation: 99}, symbols)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	_, err = bitVectorSMT(readNode("missing", 8), symbols)
	requireInternalCode(t, err, "undeclared_variable")
	_, err = binarySMT("bvsub", &bitVectorNode{left: nil}, symbols)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	badRight := &bitVectorNode{left: readNode("value", 8), right: nil}
	_, err = binarySMT("bvsub", badRight, symbols)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
}

func TestDefensiveBooleanSMTErrors(t *testing.T) {
	symbols := map[string]string{"value": "|value|"}
	_, err := booleanSMT(BooleanExpression{}, symbols)
	requireInternalCode(t, err, "unknown_boolean_semantics")
	expression := BooleanExpression{operation: unsignedLessThan, right: BitVectorExpression{node: readNode("value", 8)}}
	_, err = booleanSMT(expression, symbols)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	expression.left = expression.right
	expression.right = BitVectorExpression{}
	_, err = booleanSMT(expression, symbols)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
}
