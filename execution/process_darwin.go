// macOS supplies byte-valued resident-memory peaks and process-group cancellation.
// This measurement does not account for descendant processes or enforce RSS limits.
package execution

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}

func memoryPeaks(state *os.ProcessState) (int64, int64, error) {
	worker, valid := state.SysUsage().(*syscall.Rusage)
	if !valid {
		return 0, 0, errors.New("worker resident-memory measurement is unavailable")
	}
	var observer syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &observer); err != nil {
		return 0, 0, fmt.Errorf("observer resident-memory measurement: %w", err)
	}
	return worker.Maxrss, observer.Maxrss, nil
}
