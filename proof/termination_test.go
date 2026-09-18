// Termination tests distinguish terminal stutter, deadlock, and infinite cycles.
// They never classify a nonterminal deadlock as successful completion.
package proof_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/proof"
)

func TestNonterminationCycle(t *testing.T) {
	query := terminationQuery("environment")
	result, err := proof.Check(validatedFixture(t), query)
	if err != nil {
		t.Fatalf("proof.Check() error = %v", err)
	}
	if result.Witness == nil || result.Witness.Kind != proof.WitnessTerminationCycle {
		t.Fatalf("proof.Check() = %#v", result)
	}
	wantStates := []string{"work", "work"}
	gotStates := []string{result.Witness.Cycle.States[0].ID, result.Witness.Cycle.States[1].ID}
	if !reflect.DeepEqual(gotStates, wantStates) ||
		!reflect.DeepEqual(result.Witness.Cycle.TransitionIDs, []string{"synthetic-step"}) {
		t.Errorf("cycle = %#v", result.Witness.Cycle)
	}
}

func TestTerminalStutterProves(t *testing.T) {
	query := terminationQuery("work")
	result, err := proof.Check(validatedFixture(t), query)
	if err != nil {
		t.Fatalf("proof.Check() error = %v", err)
	}
	if result.Verdict != proof.VerdictProved || result.Witness != nil {
		t.Fatalf("proof.Check() = %#v", result)
	}
}

func TestNonterminalDeadlock(t *testing.T) {
	validated := modifiedFixture(t, makeEnvironmentDeadlock)
	query := terminationQuery("work")
	query.RootIDs = []string{"worker-root"}
	result, err := proof.Check(validated, query)
	if err != nil {
		t.Fatalf("proof.Check() error = %v", err)
	}
	if result.Witness == nil || result.Witness.Kind != proof.WitnessTerminationDeadlock ||
		result.Witness.StateID != "environment" {
		t.Fatalf("proof.Check() = %#v", result)
	}
}

func terminationQuery(terminalID string) proof.Query {
	return proof.Query{
		RootIDs: []string{"main-root"},
		Requirement: proof.Requirement{
			ID: "must-terminate", Kind: proof.RequirementTotalTermination,
			TerminalStateIDs: []string{terminalID},
		},
	}
}

func makeEnvironmentDeadlock(request *fixtureRequest) {
	request.Model.Transitions[2].FromStateID = "start"
}
