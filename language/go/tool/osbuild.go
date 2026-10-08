// Purpose: names the exact operating-system build the code was built on.
// Never:   reports a version family where the exact build is available.
package tool

import (
	"fmt"
	"runtime"
	"strings"
)

// OsBuild is the OS build, e.g. `macOS 27.0 (25A123)` or `Linux 6.8.0-45-generic`.
// System-call certificates are keyed by this, so a family name such as
// "macOS 27" would let one build's certificate cover another.
func OsBuild(root string) (string, error) {
	if runtime.GOOS != "darwin" {
		release, err := Printed(root, "uname", "-sr")
		return strings.TrimSpace(release), err
	}
	version, err := Printed(root, "sw_vers", "-productVersion")
	if err != nil {
		return "", err
	}
	build, err := Printed(root, "sw_vers", "-buildVersion")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("macOS %s (%s)", strings.TrimSpace(version), strings.TrimSpace(build)), nil
}
