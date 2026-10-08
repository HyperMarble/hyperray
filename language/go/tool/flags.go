// Purpose: reject build flags that could change unrecorded inputs or replace
// the adapter's own package overlay and output paths.
// Never:   report a build whose pin hashes describe different module files.
package tool

import (
	"fmt"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

func ValidateChoice(root string, choice record.Choice) error {
	for _, flag := range choice.Flags {
		if protectedFlag(flag, true) {
			return record.Blocked{Reason: fmt.Sprintf("unsupported requested build flag %q", flag)}
		}
	}
	value, err := Printed(root, "go", "env", "GOFLAGS")
	if err != nil {
		return err
	}
	for _, flag := range strings.Fields(value) {
		if protectedFlag(flag, false) {
			return record.Blocked{Reason: fmt.Sprintf("unsupported GOFLAGS build flag %q", flag)}
		}
	}
	return nil
}

func protectedFlag(flag string, _ bool) bool {
	name, value, hasValue := strings.Cut(strings.TrimLeft(flag, "-"), "=")
	switch name {
	case "modfile", "overlay", "C":
		return true
	case "mod":
		return !hasValue || value == "mod"
	}
	return false
}

// BuildFlags is every flag the build ran with, in the order Go reads them:
// GOFLAGS from the environment first, then what was requested.
func BuildFlags(root string, choice record.Choice) ([]string, error) {
	value, err := Printed(root, "go", "env", "GOFLAGS")
	if err != nil {
		return nil, err
	}
	return append(strings.Fields(value), choice.Args()...), nil
}
