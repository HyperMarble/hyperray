// Purpose: records the exact go tool the module builds with.
// Never:   reports the machine's default go when the module's `toolchain`
// line or GOTOOLCHAIN selects another.
package tool

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// GoToolchain asks the go tool that root selects for its version, host and
// binary. `go version` prints `go version go1.26.0 darwin/arm64`.
func GoToolchain(root string) (record.Toolchain, error) {
	text, err := Printed(root, "go", "version")
	if err != nil {
		return record.Toolchain{}, err
	}
	fields := strings.Fields(text)
	if len(fields) < 4 {
		return record.Toolchain{}, record.Unreadable("go version", errors.New("no version and host"))
	}
	goroot, err := Printed(root, "go", "env", "GOROOT")
	if err != nil {
		return record.Toolchain{}, err
	}
	compiler, err := record.DigestOf(filepath.Join(strings.TrimSpace(goroot), "bin", "go"))
	if err != nil {
		return record.Toolchain{}, err
	}
	return record.Toolchain{Version: fields[2], Host: fields[3], Compiler: compiler}, nil
}
