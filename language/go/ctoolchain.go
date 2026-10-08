// Purpose: records the C compiler cgo uses, and on macOS the Apple SDK, as
// the system's own tools name them, when cgo is switched on.
// Never:   guesses a compiler: the go tool names it, the system locates it,
// or the build is blocked.
package goadapter

import (
	"runtime"
	"strings"
)

// cToolchain is nil when CGO_ENABLED is not 1: no C compiler takes part.
func cToolchain(root string, environment []EnvVar) (*CToolchain, error) {
	if setting(environment, "CGO_ENABLED") != "1" {
		return nil, nil
	}
	compiler := strings.Fields(setting(environment, "CC"))
	if len(compiler) == 0 {
		return nil, nil
	}
	if runtime.GOOS == "darwin" {
		return apple(root, compiler[0])
	}
	path, err := printed(root, "sh", "-c", "command -v "+compiler[0])
	if err != nil {
		return nil, err
	}
	return describe(root, strings.TrimSpace(path), compiler[0], nil, nil)
}

// apple asks xcrun, which is what `cc` on macOS goes through.
func apple(root, name string) (*CToolchain, error) {
	path, err := printed(root, "xcrun", "--find", name)
	if err != nil {
		return nil, err
	}
	sdkPath, err := printed(root, "xcrun", "--show-sdk-path")
	if err != nil {
		return nil, err
	}
	sdkVersion, err := printed(root, "xcrun", "--show-sdk-version")
	if err != nil {
		return nil, err
	}
	return describe(root, strings.TrimSpace(path), name, trimmed(sdkPath), trimmed(sdkVersion))
}

func describe(root, path, name string, sdkPath, sdkVersion *string) (*CToolchain, error) {
	version, err := printed(root, name, "--version")
	if err != nil {
		return nil, err
	}
	digest, err := digestOf(path)
	if err != nil {
		return nil, err
	}
	return &CToolchain{Compiler: digest, Version: firstLine(version), SdkPath: sdkPath, SdkVersion: sdkVersion}, nil
}

func trimmed(text string) *string {
	value := strings.TrimSpace(text)
	return &value
}

func firstLine(text string) string {
	return strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
}
