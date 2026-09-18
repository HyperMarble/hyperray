// These tests cover defensive miter failures behind immutable public artifacts.
// They never permit callers to forge an accepted miter digest.
package circuit

import "testing"

func TestDefensiveMiterErrors(t *testing.T) {
	_, err := finalizedMiter("", nil, nil, engineError("emission_error", "fixture"))
	requireInternalCode(t, err, "emission_error")
	err = miterError(Miter{smt2: "(check-sat)\n", digest: "sha256:wrong"})
	requireInternalCode(t, err, "miter_digest_mismatch")
}

func TestDefensiveToolDigestError(t *testing.T) {
	_, digestError := executableDigest("/hyperray/missing/tool")
	_, err := identifiedTool("tool", "version", "", digestError)
	requireInternalCode(t, err, "tool_digest_error")
}

func TestDefensiveReferenceEmissionError(t *testing.T) {
	valid, invalid := internalRelations()
	_, _, _, err := writeMiter(invalid, valid)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
}

func TestDefensiveCandidateEmissionError(t *testing.T) {
	valid, invalid := internalRelations()
	_, _, _, err := writeMiter(valid, invalid)
	requireInternalCode(t, err, "unknown_bit_vector_semantics")
}

func TestDefensiveObservationEmissionError(t *testing.T) {
	input := Variable{name: "input", width: 8}
	output := Observation{name: "flag", value: BooleanExpression{}}
	relation := StepRelation{inputs: []Variable{input}, observations: []Observation{output}}
	queries := relationOutputs(relation)
	_, err := definitionLines(relation, queries, map[string]string{"input": "|input|"}, true)
	requireInternalCode(t, err, "unknown_boolean_semantics")
}

func internalRelations() (StepRelation, StepRelation) {
	state := Variable{name: "state", width: 8}
	validOutput := NextState{name: "state", value: Read(state)}
	invalidOutput := NextState{name: "state", value: BitVectorExpression{}}
	valid := StepRelation{states: []Variable{state}, nextStates: []NextState{validOutput}}
	invalid := StepRelation{states: []Variable{state}, nextStates: []NextState{invalidOutput}}
	return valid, invalid
}
