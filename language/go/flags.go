// Purpose: reject build flags that could change unrecorded inputs or replace
// the adapter's own package overlay and output paths.
// Never:   report a build whose pin hashes describe different module files.
package goadapter

import (
	"fmt"
	"strings"
)

func validateChoice(root string, choice Choice) error {
	for _, flag := range choice.Flags {
		if protectedFlag(flag, true) {
			return Blocked{fmt.Sprintf("unsupported requested build flag %q", flag)}
		}
	}
	value, err := printed(root, "go", "env", "GOFLAGS")
	if err != nil {
		return err
	}
	for _, flag := range strings.Fields(value) {
		if protectedFlag(flag, false) {
			return Blocked{fmt.Sprintf("unsupported GOFLAGS build flag %q", flag)}
		}
	}
	return nil
}

func protectedFlag(flag string, requested bool) bool {
	name, value, hasValue := strings.Cut(strings.TrimLeft(flag, "-"), "=")
	switch name {
	case "modfile", "overlay", "C":
		return true
	case "mod":
		return !hasValue || value == "mod"
	case "tags", "o":
		return requested
	}
	return false
}
