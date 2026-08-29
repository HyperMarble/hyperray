// Where a task keeps its config. New tasks use hyperray.toml; old task
// folders still carry hyperray.toml, so we fall back to it instead of breaking
// them.
package main

import (
	"os"
	"path/filepath"
)

func taskConfigPath(taskDir string) string {
	primary := filepath.Join(taskDir, "hyperray.toml")
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	return filepath.Join(taskDir, "hyperray.toml")
}
