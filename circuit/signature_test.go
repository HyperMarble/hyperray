// Signature tests reject different state, input, and observation boundaries.
// They never ask a solver to compare relations with different domains.
package circuit_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestRelationValidationBeforeSignatureComparison(t *testing.T) {
	valid := balanceRelation(t, 8, "unsigned_less_than")
	_, err := circuit.BuildMiter(circuit.StepRelation{}, valid)
	requireEngineError(t, err, "no_outputs", "relation")
	_, err = circuit.BuildMiter(valid, circuit.StepRelation{})
	requireEngineError(t, err, "no_outputs", "relation")
}

func TestRelationValidationRejectsStateSignatureMismatch(t *testing.T) {
	reference := balanceRelation(t, 8, "unsigned_less_than")
	candidate := balanceRelation(t, 16, "unsigned_less_than")
	_, err := circuit.BuildMiter(reference, candidate)
	requireEngineError(t, err, "state_signature_mismatch", "relations")
	state := mustVariable(t, "other", 8)
	input := mustVariable(t, "cost", 8)
	candidate = namedRelation(t, state, input, "insufficient")
	_, err = circuit.BuildMiter(reference, candidate)
	requireEngineError(t, err, "state_signature_mismatch", "relations")
	candidate = twoStateRelation(t)
	_, err = circuit.BuildMiter(reference, candidate)
	requireEngineError(t, err, "state_signature_mismatch", "relations")
}

func TestRelationValidationRejectsInputSignatureMismatch(t *testing.T) {
	state := mustVariable(t, "balance", 8)
	reference := namedRelation(t, state, mustVariable(t, "cost", 8), "flag")
	candidate := namedRelation(t, state, mustVariable(t, "cost", 16), "flag")
	_, err := circuit.BuildMiter(reference, candidate)
	requireEngineError(t, err, "input_signature_mismatch", "relations")
}

func TestRelationValidationRejectsObservationSignatureMismatch(t *testing.T) {
	state := mustVariable(t, "balance", 8)
	input := mustVariable(t, "cost", 8)
	reference := namedRelation(t, state, input, "first")
	candidate := namedRelation(t, state, input, "second")
	_, err := circuit.BuildMiter(reference, candidate)
	requireEngineError(t, err, "observation_signature_mismatch", "relations")
	candidate = nextOnlyRelation(t, state, input, circuit.Read(state))
	_, err = circuit.BuildMiter(reference, candidate)
	requireEngineError(t, err, "observation_signature_mismatch", "relations")
}

func twoStateRelation(t *testing.T) circuit.StepRelation {
	t.Helper()
	balance := mustVariable(t, "balance", 8)
	extra := mustVariable(t, "extra", 8)
	cost := mustVariable(t, "cost", 8)
	return orderedRelation(t,
		[]circuit.Variable{balance, extra}, []circuit.Variable{cost},
		[]circuit.NextState{mustNext(t, "balance", circuit.Read(balance)),
			mustNext(t, "extra", circuit.Read(extra))},
		[]circuit.Observation{mustObservation(t, "insufficient",
			comparisonExpression(t, "unsigned_less_than", circuit.Read(balance), circuit.Read(cost)))})
}
