// Resource outcomes override successful-looking worker output.
// An unknown execution status must not become a completed search.
package nativecheck

import (
	"fmt"
	"strings"

	"github.com/HyperMarble/hyperray/execution"
)

func stopped(result execution.Result) (string, error) {
	switch result.Status {
	case execution.Exited:
		if result.ExitCode != 0 {
			return fmt.Sprintf("checker exited with code %d", result.ExitCode), nil
		}
		return "", nil
	case execution.TimedOut, execution.Canceled, execution.OutputLimit, execution.MemoryLimit, execution.WorkerError:
		return "checker stopped: " + string(result.Status), nil
	default:
		return "", fmt.Errorf("unknown execution status %q", result.Status)
	}
}

func partialSearch(output string) string {
	for _, line := range strings.Split(output, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "warning:") || strings.Contains(lower, "error:") {
			return line
		}
	}
	return ""
}

func requiredLine(output, expected string) error {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Join(strings.Fields(line), " ") == expected {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("expected one %q line, got %d", expected, count)
	}
	return nil
}
