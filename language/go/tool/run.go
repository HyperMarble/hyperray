// Purpose: runs one tool in a folder and returns what it printed.
// Never:   treats a tool that failed, or could not start, as a success.
package tool

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// Printed runs tool with args inside folder and returns its standard output.
// A failure carries everything the tool printed, so the reason is readable.
func Printed(folder, tool string, args ...string) (string, error) {
	command := exec.Command(tool, args...)
	command.Dir = folder
	var out, errs bytes.Buffer
	command.Stdout, command.Stderr = &out, &errs
	err := command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		text := strings.TrimSpace(errs.String() + out.String())
		return "", record.ToolFailed(tool+" "+strings.Join(args, " "), text)
	}
	if err != nil {
		return "", record.ToolMissing(tool, err)
	}
	return out.String(), nil
}
