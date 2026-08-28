//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package taskbundle

import (
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Killing the process group prevents a timed-out verifier or version
	// command from leaving descendants that can mutate evidence afterward.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}
