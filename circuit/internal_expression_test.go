// These tests cover defensive expression branches behind opaque public types.
// They never make invalid expression construction part of the public API.
package circuit

import "testing"

func TestDefensiveExpressionErrors(t *testing.T) {
	_, err := nodeWidth(nil)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	_, err = nodeWidth(&bitVectorNode{operation: 99})
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	_, err = binaryWidth(&bitVectorNode{left: nil, right: readNode("right", 8)})
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	_, err = binaryWidth(&bitVectorNode{left: readNode("left", 8), right: nil})
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	_, err = binaryWidth(&bitVectorNode{left: readNode("left", 8), right: readNode("right", 16)})
	requireInternalCode(t, err, "width_mismatch")
	err = variableError(Variable{name: "value"}, "variable")
	requireInternalCode(t, err, "zero_width")
}

func TestDefensiveReferenceErrors(t *testing.T) {
	declarations := map[string]uint16{"value": 8}
	err := expressionReferencesError(nil, declarations)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	err = expressionReferencesError(&bitVectorNode{operation: 99}, declarations)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	badRight := &bitVectorNode{operation: subtractValues, left: readNode("value", 8)}
	err = expressionReferencesError(badRight, declarations)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
	badLeft := &bitVectorNode{operation: subtractValues, right: readNode("value", 8)}
	err = expressionReferencesError(badLeft, declarations)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
}
