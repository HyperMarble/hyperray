//go:build preparation_integration && darwin

// Operate freshly prepared checkers through the same public execution observer.
// A partial search remains a partial search even when its process exits zero.
package execution_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/execution"
)

func TestPreparationMatrix(t *testing.T) {
	root, preparer, tools := preparationTools(t)
	cases := preparationCases(root, t.TempDir(), tools)
	var maximum int64
	completed := 0
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			prepared := prepareNative(t, preparer, item.request)
			result := observePrepared(t, prepared, item.partial, item.broken)
			peak := validatePreparationOutput(t, item, prepared, result)
			maximum = max(maximum, peak)
			completed++
		})
	}
	t.Logf("PREPARATION_MATRIX completed=%d declared=%d peak_bytes=%d", completed, len(cases), maximum)
}

func validatePreparationOutput(t *testing.T, item preparationCase, prepared preparedResult, result execution.Result) int64 {
	t.Helper()
	if item.partial {
		if prepared.InputBits != 64 || !strings.Contains(result.Output, "max search depth too small") {
			t.Fatalf("full-width bound or partial-search evidence lost: %+v", result)
		}
		return result.CombinedPeakBytes
	}
	if item.broken {
		replay := replayCounterexample(t, prepared, result)
		return max(result.CombinedPeakBytes, replay.CombinedPeakBytes)
	}
	for _, marker := range []string{"errors: 0", fmt.Sprintf("OBSERVATIONS count=%d overflow=0", item.observations)} {
		if !strings.Contains(result.Output, marker) {
			t.Fatalf("missing %q: %s", marker, result.Output)
		}
	}
	if result.ExitCode != 0 || strings.Contains(result.Output, "Search not completed") {
		t.Fatalf("correct case did not finish: %+v", result)
	}
	return result.CombinedPeakBytes
}

func replayCounterexample(t *testing.T, prepared preparedResult, result execution.Result) execution.Result {
	t.Helper()
	if !strings.Contains(result.Output, "COUNTEREXAMPLE input=23") {
		t.Fatal(result.Output)
	}
	request := prepared.Execution
	request.Executable = prepared.Replay
	request.Arguments = []string{"23"}
	replay, err := execution.Run(context.Background(), request)
	if err != nil || replay.Status != execution.Exited || replay.ExitCode != 1 || !strings.Contains(replay.Output, "COUNTEREXAMPLE input=23") {
		t.Fatalf("replay = %+v, error = %v", replay, err)
	}
	return replay
}
