// Both execution stages must receive the caller's instruction-visit guard.
// A resource guard must not select the native path-discard mode.
package isla

import (
	"slices"
	"strconv"
	"testing"
)

func TestExecutionStagesReceiveVisitLimit(t *testing.T) {
	for _, limit := range []uint64{2, 4, 17} {
		query := Request{pcVisitLimit: limit}
		request := VerificationRequest{query: query}
		requireVisitArgument(t, query.arguments(), limit)
		requireVisitArgument(t, request.semanticArguments(), limit)
	}
}

func TestSolverStageReceivesIndependentWorkerCount(t *testing.T) {
	request := Request{workerCount: 1, pcVisitLimit: 2}
	arguments := request.arguments()
	requireWorkerArgument(t, arguments, 1)
	if arguments[slices.Index(arguments, "--pc-limit")+1] != "2" {
		t.Fatalf("visit limit changed with worker count: %v", arguments)
	}
}

func requireWorkerArgument(t *testing.T, arguments []string, want uint64) {
	t.Helper()
	position := slices.Index(arguments, "-T")
	if position < 0 || position+1 >= len(arguments) {
		t.Fatalf("missing worker count: %v", arguments)
	}
	if arguments[position+1] != strconv.FormatUint(want, 10) {
		t.Errorf("worker count = %v, want %d", arguments, want)
	}
}

func requireVisitArgument(t *testing.T, arguments []string, limit uint64) {
	t.Helper()
	position := slices.Index(arguments, "--pc-limit")
	if position < 0 || position+1 >= len(arguments) {
		t.Fatalf("missing visit guard: %v", arguments)
	}
	if arguments[position+1] != strconv.FormatUint(limit, 10) || slices.Contains(arguments, "discard") {
		t.Errorf("incorrect visit guard: %v", arguments)
	}
}
