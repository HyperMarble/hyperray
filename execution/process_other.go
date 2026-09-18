//go:build !darwin

// Unsupported platforms must stop before execution, not guess memory units.
package execution

import (
	"errors"
	"os"
	"os/exec"
)

func configureProcess(command *exec.Cmd) error {
	return errors.New("execution observer requires macOS memory accounting")
}

func memoryPeaks(state *os.ProcessState) (int64, int64, error) {
	return 0, 0, errors.New("execution observer requires macOS memory accounting")
}
