package cpp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func cppWorkspaceEnvironment(workspace semanticir.WorkspaceRef) []string {
	result := make([]string, len(workspace.Environment))
	for index, variable := range workspace.Environment {
		result[index] = variable.Name + "=" + variable.Value
	}
	return result
}

func configureCPPCommand(command *exec.Cmd, workspace semanticir.WorkspaceRef) {
	command.Env = cppWorkspaceEnvironment(workspace)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = 250 * time.Millisecond
}

func workspaceLookPath(name string, workspace semanticir.WorkspaceRef) (string, bool) {
	if filepath.IsAbs(name) {
		info, err := os.Stat(name)
		return name, err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
	}
	pathValue := ""
	for _, variable := range workspace.Environment {
		if variable.Name == "PATH" {
			pathValue = variable.Value
			break
		}
	}
	for _, directory := range filepath.SplitList(pathValue) {
		if directory == "" || !filepath.IsAbs(directory) {
			continue
		}
		candidate := filepath.Join(directory, name)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
				candidate = resolved
			}
			return filepath.Clean(candidate), true
		}
	}
	return "", false
}

func validateCPPEnvironment(workspace semanticir.WorkspaceRef) bool {
	digest, err := semanticir.Digest(workspace.Environment)
	if err != nil || workspace.EnvironmentDigest != digest || !workspace.ClearEnvironment || !workspace.KillProcessGroup {
		return false
	}
	previous := ""
	for index, variable := range workspace.Environment {
		if variable.Name == "" || strings.ContainsRune(variable.Name, '=') || strings.ContainsRune(variable.Name, '\x00') || strings.ContainsRune(variable.Value, '\x00') || index > 0 && variable.Name <= previous {
			return false
		}
		previous = variable.Name
	}
	return true
}
