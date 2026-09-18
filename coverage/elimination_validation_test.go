// Elimination validation tests reject stale operations, proofs, and transitions.
// They never reach the proof boundary with incomplete structural evidence.
package coverage_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestEliminationOperationAndProofFailures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testRequest)
		code   string
	}{
		{"unknown operation", func(request *testRequest) {
			request.Inventory.EliminationRecords[0].OperationID = "missing"
		}, "unknown_reference"},
		{"mapped operation", func(request *testRequest) {
			request.Inventory.Operations[len(request.Inventory.Operations)-1].Disposition = coverage.DispositionMapped
		}, "wrong_operation_disposition"},
		{"unknown proof", func(request *testRequest) {
			request.Inventory.EliminationRecords[0].ProofID = "proof:missing"
		}, "unknown_reference"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := completeRequest(t)
			addEliminationClaim(&request)
			testCase.change(&request)
			requireCoverageError(t, request, testCase.code)
		})
	}
}

func TestEliminationTransitionFailures(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	request.Inventory.EliminationRecords[0].EquivalentTransitionIDs = nil
	requireCoverageError(t, request, "empty_field")
	request = completeRequest(t)
	addEliminationClaim(&request)
	request.Inventory.EliminationRecords[0].EquivalentTransitionIDs = []string{"missing"}
	requireCoverageError(t, request, "unknown_reference")
}

func TestEliminatedOperationCannotHaveMachineBinding(t *testing.T) {
	request := completeRequest(t)
	addEliminationClaim(&request)
	binding := request.Inventory.MachineBindings[0]
	binding.OperationID = "optimized"
	request.Inventory.MachineBindings = append(request.Inventory.MachineBindings, binding)
	requireCoverageError(t, request, "wrong_operation_disposition")
}
