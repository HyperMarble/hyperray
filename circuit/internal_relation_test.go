// These tests cover relation defenses behind validated public output types.
// They never expose invalid output construction to external callers.
package circuit

import "testing"

func TestDefensiveRelationErrors(t *testing.T) {
	declarations := map[string]uint16{"state": 8}
	invalidName := Observation{name: "bad name"}
	err := observationsError([]Observation{invalidName}, declarations)
	requireInternalCode(t, err, "invalid_identifier")
	err = booleanReferencesError(BooleanExpression{}, declarations)
	requireInternalCode(t, err, "unknown_boolean_semantics")
	state := Variable{name: "state", width: 8}
	relation := StepRelation{
		states:     []Variable{state},
		nextStates: []NextState{{name: "state"}},
	}
	err = nextStatesError(relation, declarations)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
}
