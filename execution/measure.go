// Measurement joins the observer and worker peaks after output collection.
// Worker errors must retain their exit code and original diagnostics.
package execution

import (
	"errors"
	"fmt"
	"math"
	"os/exec"
	"time"
)

func measuredResult(command *exec.Cmd, request Request, output *boundedOutput,
	contextError, waitError error, started time.Time) (Result, error) {
	if command.ProcessState == nil {
		return Result{}, errors.New("worker process state is unavailable")
	}
	result := Result{ExitCode: command.ProcessState.ExitCode(), Output: output.content.String()}
	worker, observer, err := memoryPeaks(command.ProcessState)
	if err != nil {
		return result, fmt.Errorf("measure checker: %w", err)
	}
	combined, err := combinedPeak(worker, observer)
	if err != nil {
		return result, err
	}
	result.WorkerPeakBytes = worker
	result.ObserverPeakBytes = observer
	result.CombinedPeakBytes = combined
	result.MemoryBudgetBytes = request.MemoryBudgetBytes
	result.Elapsed = time.Since(started)
	result.Status = outcome(contextError, output.exceeded, combined, request.MemoryBudgetBytes)
	if waitError != nil {
		result.Diagnostic = waitError.Error()
	}
	var exitError *exec.ExitError
	if result.Status == Exited && waitError != nil && !errors.As(waitError, &exitError) {
		result.Status = WorkerError
	}
	return result, nil
}

func combinedPeak(worker, observer int64) (int64, error) {
	if worker <= 0 || observer <= 0 {
		return 0, errors.New("resident-memory measurements must be positive")
	}
	if worker > math.MaxInt64-observer {
		return 0, errors.New("combined resident-memory measurement overflows")
	}
	return worker + observer, nil
}
