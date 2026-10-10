// Purpose: records the go tool the module builds with, as go itself reports
// it: version, host and experiments, and the go command by hash.
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
	settings, err := Printed(root, "go", "env", "GOROOT", "GOEXPERIMENT")
	if err != nil {
		return record.Toolchain{}, err
	}
	lines := strings.SplitN(strings.TrimRight(settings, "\n")+"\n", "\n", 3)
	goroot, experiments := lines[0], lines[1]
	compiler, err := record.DigestOf(filepath.Join(goroot, "bin", "go"))
	if err != nil {
		return record.Toolchain{}, err
	}
	return record.Toolchain{Version: fields[2], Host: fields[3], Experiments: experiments, Compiler: compiler}, nil
}
