package rust

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// configureRustCommand applies the frozen execution contract to every Rust
// compiler, prover, and generated-program process. In particular, cancellation
// terminates the complete process group rather than leaving compiler/linker
// children running outside the recorded evidence boundary.
func configureRustCommand(command *exec.Cmd, workspace semanticir.WorkspaceRef) {
	command.Env = rustWorkspaceEnvironment(workspace)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = 250 * time.Millisecond
}
