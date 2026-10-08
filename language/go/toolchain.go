// Purpose: records the exact go tool the module builds with.
// Never:   reports the machine's default go when the module's `toolchain`
// line or GOTOOLCHAIN selects another.
package goadapter

import (
	"errors"
	"path/filepath"
	"strings"
)

// toolchain asks the go tool that root selects for its version, host and
// binary. `go version` prints `go version go1.26.0 darwin/arm64`.
func toolchain(root string) (Toolchain, error) {
	text, err := printed(root, "go", "version")
	if err != nil {
		return Toolchain{}, err
	}
	fields := strings.Fields(text)
	if len(fields) < 4 {
		return Toolchain{}, unreadable("go version", errors.New("no version and host"))
	}
	goroot, err := printed(root, "go", "env", "GOROOT")
	if err != nil {
		return Toolchain{}, err
	}
	compiler, err := digestOf(filepath.Join(strings.TrimSpace(goroot), "bin", "go"))
	if err != nil {
		return Toolchain{}, err
	}
	return Toolchain{Version: fields[2], Host: fields[3], Compiler: compiler}, nil
}
