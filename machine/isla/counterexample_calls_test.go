// Counterexample call values must be Boolean, complete, and requested.
// Ordinary register values cannot substitute for call evidence.
package isla

import "testing"

func TestCounterexampleCallObservations(t *testing.T) {
	request, semantics, proposal := matchingFixture()
	request.query.forbiddenModelCalls = []string{"fault"}
	proposal.Status = CounterexampleFound
	for _, state := range []string{"called:fault=true;", "0:x10=17;called:fault=false;"} {
		proposal.CounterexampleState = state
		result, err := verifiedResult(request, semantics, proposal)
		if err != nil {
			t.Fatal(err)
		}
		called, found := result.ModelCalls["fault"]
		if !found || called != (state == "called:fault=true;") {
			t.Errorf("call observation differs: %#v", result.ModelCalls)
		}
	}
}

func TestCounterexampleRejectsMissingCallEvidence(t *testing.T) {
	request, semantics, proposal := matchingFixture()
	request.query.forbiddenModelCalls = []string{"fault"}
	proposal.Status = CounterexampleFound
	for _, state := range []string{
		"0:x10=17;", "called:fault=1;", "called:fault=v7;", "called:fault;",
		"called:other=true;", "called:fault=true;called:fault=false;",
	} {
		proposal.CounterexampleState = state
		requireMatchingFailure(t, request, semantics, proposal, ProtocolError)
	}
}
