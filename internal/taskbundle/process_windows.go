//go:build windows

package taskbundle

import (
	"os/exec"
	"strconv"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func terminateProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// taskkill /T terminates the process tree. Process.Kill remains the
	// fallback when taskkill is unavailable or the process already exited.
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	_ = cmd.Process.Kill()
}
