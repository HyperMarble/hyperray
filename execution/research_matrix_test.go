//go:build execution_integration && darwin

// Real precompiled research checkers establish SDK transport and measured cost.
// These tests do not establish semantic coverage or compiler provenance.
package execution_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/execution"
)

func TestResearchMatrix(t *testing.T) {
	root, err := filepath.Abs("../../hyperray-research")
	if err != nil {
		t.Fatal(err)
	}
	cases := nativeCases(filepath.Join(root, "native-exhaustive", "results", "final"))
	cases = append(cases, threadCases(filepath.Join(root, "thread-exhaustive"))...)
	var maximum int64
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			result := observeResearchCase(t, item)
			maximum = max(maximum, result.CombinedPeakBytes)
		})
	}
	t.Logf("SDK_MATRIX cases=%d peak_bytes=%d budget_bytes=%d", len(cases), maximum,
		execution.MaximumMemoryBudgetBytes)
}

func observeResearchCase(t *testing.T, item researchCase) execution.Result {
	t.Helper()
	request := execution.Request{Executable: item.executable, Arguments: item.arguments,
		Directory: t.TempDir(), Environment: []string{}, Timeout: 10 * time.Second,
		OutputLimitBytes: 1_000_000, MemoryBudgetBytes: execution.MaximumMemoryBudgetBytes}
	result, err := execution.Run(context.Background(), request)
	if err != nil || result.Status != execution.Exited {
		t.Fatalf("execution = %+v, error = %v", result, err)
	}
	if (result.ExitCode != 0) != item.nonzero {
		t.Fatalf("unexpected worker exit: %+v", result)
	}
	for _, marker := range item.markers {
		if !strings.Contains(result.Output, marker) {
			t.Errorf("missing %q in %q", marker, result.Output)
		}
	}
	t.Logf("exit=%d worker=%d observer=%d combined=%d", result.ExitCode,
		result.WorkerPeakBytes, result.ObserverPeakBytes, result.CombinedPeakBytes)
	return result
}
