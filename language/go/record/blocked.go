// Purpose: every reason the adapter can give for not producing a build.
// Never:   hides a reason; each one reaches the user as readable text.
package record

import "fmt"

// Blocked is why there is no build record.
type Blocked struct{ Reason string }

func (blocked Blocked) Error() string { return blocked.Reason }

func NoModule(root string) Blocked {
	return Blocked{fmt.Sprintf("%s has no go.mod: which Go version and modules should be used?", root)}
}

func ToolMissing(tool string, cause error) Blocked {
	return Blocked{fmt.Sprintf("%s could not start: %v", tool, cause)}
}

func ToolFailed(tool, output string) Blocked {
	return Blocked{fmt.Sprintf("%s failed: %s", tool, output)}
}

func Unreadable(what string, cause error) Blocked {
	return Blocked{fmt.Sprintf("cannot read %s: %v", what, cause)}
}
