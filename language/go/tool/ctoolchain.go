// Purpose: records the C compiler cgo uses, and on macOS the Apple SDK, as
// the system's own tools name them, when cgo is switched on.
// Never:   guesses a compiler: the go tool names it, the system locates it,
// or the build is blocked.
package tool

import (
	"errors"
	"runtime"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

var errNotFound = errors.New("not found on this machine")

// CCompiler is nil when CGO_ENABLED is not 1: no C compiler takes part.
// Otherwise it is the C compiler Go names, and every other tool beside it.
func CCompiler(root string, environment []record.EnvVar) (*record.CToolchain, error) {
	if EnvValue(environment, "CGO_ENABLED") != "1" {
		return nil, nil
	}
	compiler := strings.Fields(EnvValue(environment, "CC"))
	if len(compiler) == 0 {
		return nil, nil
	}
	found, err := cCompiler(root, compiler[0])
	if err != nil {
		return nil, err
	}
	return found, otherTools(root, environment, compiler[0], found)
}

func cCompiler(root, name string) (*record.CToolchain, error) {
	if runtime.GOOS == "darwin" {
		return apple(root, name)
	}
	path, ok := locate(root, name)
	if !ok {
		return nil, record.ToolMissing(name, errNotFound)
	}
	return describe(root, path, name, nil, nil)
}

// apple asks xcrun, which is what `cc` on macOS goes through.
func apple(root, name string) (*record.CToolchain, error) {
	path, ok := locate(root, name)
	if !ok {
		return nil, record.ToolMissing(name, errNotFound)
	}
	sdkPath, err := Printed(root, "xcrun", "--show-sdk-path")
	if err != nil {
		return nil, err
	}
	sdkVersion, err := Printed(root, "xcrun", "--show-sdk-version")
	if err != nil {
		return nil, err
	}
	return describe(root, path, name, trimmed(sdkPath), trimmed(sdkVersion))
}

func describe(root, path, name string, sdkPath, sdkVersion *string) (*record.CToolchain, error) {
	version, err := Printed(root, name, "--version")
	if err != nil {
		return nil, err
	}
	digest, err := record.DigestOf(path)
	if err != nil {
		return nil, err
	}
	return &record.CToolchain{Compiler: digest, Version: firstLine(version), SdkPath: sdkPath, SdkVersion: sdkVersion}, nil
}

func trimmed(text string) *string {
	value := strings.TrimSpace(text)
	return &value
}

func firstLine(text string) string {
	return strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
}
