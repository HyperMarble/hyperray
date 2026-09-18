// Resource tests force real observer stop conditions with bounded fixtures.
// Neither time nor output exhaustion can count as a successful observation.
package execution_test

import (
	"context"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/execution"
)

func TestTimeout(t *testing.T) {
	request := fixtureRequest(t, "wait")
	request.Timeout = 50 * time.Millisecond
	result, err := execution.Run(context.Background(), request)
	if err != nil || result.Status != execution.TimedOut {
		t.Fatalf("timeout result = %+v, error = %v", result, err)
	}
}

func TestOutputLimit(t *testing.T) {
	request := fixtureRequest(t, "flood")
	request.OutputLimitBytes = 32
	result, err := execution.Run(context.Background(), request)
	if err != nil || result.Status != execution.OutputLimit {
		t.Fatalf("output limit result = %+v, error = %v", result, err)
	}
	if len(result.Output) != request.OutputLimitBytes {
		t.Fatalf("retained output length = %d", len(result.Output))
	}
}

func TestMemoryLimit(t *testing.T) {
	request := fixtureRequest(t, "silent")
	request.MemoryBudgetBytes = 1
	request.OutputLimitBytes = 1
	result, err := execution.Run(context.Background(), request)
	if err != nil || result.Status != execution.MemoryLimit || result.CombinedPeakBytes <= 1 {
		t.Fatalf("memory limit result = %+v, error = %v", result, err)
	}
}

func TestExactOutputLimit(t *testing.T) {
	request := fixtureRequest(t, "environment")
	request.OutputLimitBytes = len("unset\n")
	result, err := execution.Run(context.Background(), request)
	if err != nil || result.Status != execution.Exited || result.Output != "unset\n" {
		t.Fatalf("exact output result = %+v, error = %v", result, err)
	}
}
