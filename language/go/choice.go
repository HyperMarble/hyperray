// Purpose: the build the user asks for: which build tags, and which extra
// build flags, exactly as the go tool takes them.
// Never:   switches on a tag or adds a flag nobody asked for; with no request
// it is exactly the module's own default build.
package goadapter

import "strings"

// Choice is one build of the module: its tags and extra go build flags.
type Choice struct {
	Tags  []string `json:"tags"`
	Flags []string `json:"flags"`
}

// args is what the go tool takes for this build. `-mod=readonly` makes Go
// fail rather than change go.mod or go.sum.
func (choice Choice) args() []string {
	args := []string{"-mod=readonly"}
	if len(choice.Tags) > 0 {
		args = append(args, "-tags", strings.Join(choice.Tags, ","))
	}
	return append(args, choice.Flags...)
}
