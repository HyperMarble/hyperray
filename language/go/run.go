// Purpose: runs one tool in a folder and returns what it printed.
// Never:   treats a tool that failed, or could not start, as a success.
package goadapter

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

// printed runs tool with args inside folder and returns its standard output.
// A failure carries everything the tool printed, so the reason is readable.
func printed(folder, tool string, args ...string) (string, error) {
	command := exec.Command(tool, args...)
	command.Dir = folder
	var out, errs bytes.Buffer
	command.Stdout, command.Stderr = &out, &errs
	err := command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		printed := strings.TrimSpace(errs.String() + out.String())
		return "", toolFailed(tool+" "+strings.Join(args, " "), printed)
	}
	if err != nil {
		return "", toolMissing(tool, err)
	}
	return out.String(), nil
}
