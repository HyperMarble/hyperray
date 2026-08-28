//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package executor

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
	// Kill the whole verifier process group so a timed-out shell cannot
	// leave a compiler or test worker mutating the workspace after restore.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}
