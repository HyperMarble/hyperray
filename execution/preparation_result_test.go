//go:build preparation_integration && darwin

// Every generated checker passes through the public result API.
// Test-only marker assertions cannot replace production result validation.
package execution_test

import (
	"context"
	"testing"

	"github.com/HyperMarble/hyperray/execution"
	"github.com/HyperMarble/hyperray/execution/nativecheck"
)

func observePrepared(t *testing.T, prepared preparedResult, partial, broken bool) execution.Result {
	t.Helper()
	request := nativeRequest(prepared)
	result, err := nativecheck.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("native result = %+v, error = %v", result, err)
	}
	expected := nativecheck.SearchReportedComplete
	if partial {
		expected = nativecheck.Incomplete
	}
	if broken {
		expected = nativecheck.CounterexampleReproduced
	}
	if result.Status != expected {
		t.Fatalf("expected %s, received %+v", expected, result)
	}
	if broken && (result.Replay == nil || result.Counterexample == nil) {
		t.Fatal("missing automatic replay evidence")
	}
	return result.Search
}

func nativeRequest(prepared preparedResult) nativecheck.Request {
	return nativecheck.Request{
		Search: prepared.Execution, ReplayExecutable: prepared.Replay,
		Minimum: prepared.Minimum, Maximum: prepared.Maximum,
	}
}
