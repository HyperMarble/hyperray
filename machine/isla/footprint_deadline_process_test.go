// Deadline tests observe process termination through the host process table.
// A process that still exists cannot count as reaped.
package isla_test

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func assertFootprintProcessReaped(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	processID, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(processID)
	if err != nil {
		t.Fatal(err)
	}
	err = process.Signal(syscall.Signal(0))
	if !errors.Is(err, syscall.ESRCH) && !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("footprint process %d is not reaped: %v", processID, err)
	}
	if err := process.Release(); err != nil {
		t.Fatal(err)
	}
}
