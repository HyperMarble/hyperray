// Purpose: the other programs the C toolchain uses, as Go names them in
// go env: the C++ and Fortran compilers, the archiver, pkg-config, and the
// linker the C compiler reports. Each is located the way the system
// locates it and hashed.
// Never:   guesses a program: Go names it, the system finds it, or it is
// recorded as absent.
package tool

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// otherTools fills every tool beside the C compiler into found.
func otherTools(root string, environment []record.EnvVar, cc string, found *record.CToolchain) error {
	var err error
	if found.Cxx, err = namedTool(root, EnvValue(environment, "CXX"), true); err != nil {
		return err
	}
	if found.Fortran, err = namedTool(root, EnvValue(environment, "FC"), true); err != nil {
		return err
	}
	if found.Archiver, err = namedTool(root, EnvValue(environment, "AR"), false); err != nil {
		return err
	}
	if found.PkgConfig, err = namedTool(root, EnvValue(environment, "PKG_CONFIG"), true); err != nil {
		return err
	}
	found.Linker, err = linkerOf(root, cc)
	return err
}

// namedTool is the program the first word of a go env value names, by
// hash, or nil when the value is empty or the machine does not have it.
func namedTool(root, value string, versioned bool) (*record.Tool, error) {
	words := strings.Fields(value)
	if len(words) == 0 {
		return nil, nil
	}
	path, ok := locate(root, words[0])
	if !ok {
		return nil, nil
	}
	digest, err := record.DigestOf(path)
	if err != nil {
		return nil, err
	}
	tool := &record.Tool{File: digest}
	if text, err := Printed(root, words[0], "--version"); versioned && err == nil {
		tool.Version = firstLine(text)
	}
	return tool, nil
}

// locate finds a program the way the system does: xcrun on macOS, the
// shell's command -v elsewhere.
func locate(root, name string) (string, bool) {
	text, err := Printed(root, "sh", "-c", "command -v "+name)
	if runtime.GOOS == "darwin" {
		text, err = Printed(root, "xcrun", "--find", name)
	}
	return strings.TrimSpace(text), err == nil
}

// linkerOf asks the C compiler which linker it runs.
func linkerOf(root, cc string) (*record.Tool, error) {
	text, err := Printed(root, cc, "-print-prog-name=ld")
	if err != nil {
		return nil, err
	}
	path := strings.TrimSpace(text)
	if !filepath.IsAbs(path) {
		located, ok := locate(root, path)
		if !ok {
			return nil, nil
		}
		path = located
	}
	digest, err := record.DigestOf(path)
	if err != nil {
		return nil, err
	}
	return &record.Tool{File: digest}, nil
}
