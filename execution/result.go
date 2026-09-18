// Results describe measured execution, never program correctness or search coverage.
// A zero worker exit code must not become a proof verdict.
package execution

import (
	"context"
	"errors"
	"time"
)

type Status string

const (
	Exited      Status = "exited"
	TimedOut    Status = "timed_out"
	Canceled    Status = "canceled"
	OutputLimit Status = "output_limit"
	MemoryLimit Status = "memory_limit"
	WorkerError Status = "worker_error"
)

type Result struct {
	Status            Status        `json:"status"`
	ExitCode          int           `json:"exit_code"`
	Output            string        `json:"output"`
	Diagnostic        string        `json:"diagnostic,omitempty"`
	Elapsed           time.Duration `json:"elapsed_nanoseconds"`
	WorkerPeakBytes   int64         `json:"worker_peak_bytes"`
	ObserverPeakBytes int64         `json:"observer_peak_bytes"`
	CombinedPeakBytes int64         `json:"combined_peak_upper_bound_bytes"`
	MemoryBudgetBytes int64         `json:"memory_budget_bytes"`
}

func outcome(contextError error, outputExceeded bool, peak, budget int64) Status {
	if outputExceeded {
		return OutputLimit
	}
	if errors.Is(contextError, context.DeadlineExceeded) {
		return TimedOut
	}
	if contextError != nil {
		return Canceled
	}
	if peak >= budget {
		return MemoryLimit
	}
	return Exited
}
