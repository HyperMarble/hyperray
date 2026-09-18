// Footprint resource details preserve one named cause and bounded tool output.
// They must not turn a resource failure into a proof result.
package isla

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ResourceLimitCause identifies the first observed footprint resource cause.
type ResourceLimitCause string

const (
	ResourceLimitTimeout        ResourceLimitCause = "deadline_exceeded"
	ResourceLimitCanceled       ResourceLimitCause = "context_canceled"
	ResourceLimitStdoutExceeded ResourceLimitCause = "stdout_limit_exceeded"
	ResourceLimitStderrExceeded ResourceLimitCause = "stderr_limit_exceeded"
	ResourceLimitOutputExceeded ResourceLimitCause = "output_limit_exceeded"
)

// ResourceLimitDetail records bounded process evidence for a resource error.
type ResourceLimitDetail struct {
	Cause               ResourceLimitCause
	ExitCode            int
	ElapsedMilliseconds int64
	Stdout              string
	Stderr              string
	StdoutLimit         uint64
	StderrLimit         uint64
	StdoutLimitExceeded bool
	StderrLimitExceeded bool
}

func newResourceLimitDetail(contextError error, output commandOutput, outputLimit uint64) *ResourceLimitDetail {
	return &ResourceLimitDetail{
		Cause:    resourceLimitCause(contextError, output.stdoutExceeded, output.diagnosticsExceeded),
		ExitCode: output.exitCode, ElapsedMilliseconds: output.elapsed.Milliseconds(),
		Stdout: output.stdout, Stderr: output.diagnostics,
		StdoutLimit: outputLimit, StderrLimit: outputLimit,
		StdoutLimitExceeded: output.stdoutExceeded, StderrLimitExceeded: output.diagnosticsExceeded,
	}
}

func resourceLimitCause(contextError error, stdoutExceeded bool, stderrExceeded bool) ResourceLimitCause {
	if contextError == context.DeadlineExceeded {
		return ResourceLimitTimeout
	}
	if contextError == context.Canceled {
		return ResourceLimitCanceled
	}
	if stdoutExceeded && stderrExceeded {
		return ResourceLimitOutputExceeded
	}
	if stdoutExceeded {
		return ResourceLimitStdoutExceeded
	}
	return ResourceLimitStderrExceeded
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return -1
}

func (detail *ResourceLimitDetail) summary() string {
	return fmt.Sprintf("footprint resource limit reached: cause=%s exit_code=%d elapsed_ms=%d stdout_bytes=%d stderr_bytes=%d stdout_limit=%d stderr_limit=%d", detail.Cause, detail.ExitCode, detail.ElapsedMilliseconds, len(detail.Stdout), len(detail.Stderr), detail.StdoutLimit, detail.StderrLimit)
}
