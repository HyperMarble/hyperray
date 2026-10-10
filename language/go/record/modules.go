// Purpose: the record's view of modules: what Go stamps into a built file
// about them, and the modules built or pinned beside the root one.
// Never:   holds a module fact that was not read from a file or the go tool.
package record

// Module is one module version as Go recorded it inside the built file.
// Replace is set when a `replace` line swapped in other code.
type Module struct {
	Path    string  `json:"path"`
	Version string  `json:"version"`
	Sum     string  `json:"sum,omitempty"`
	Replace *Module `json:"replace,omitempty"`
}

// Setting is one build setting as Go recorded it inside the built file.
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// BuildInfo is Go's own record of a build, read back from the built file.
type BuildInfo struct {
	GoVersion string    `json:"go_version"`
	Path      string    `json:"path"`
	Main      Module    `json:"main"`
	Deps      []Module  `json:"deps"`
	Settings  []Setting `json:"settings"`
}

// LocalModule is one locally sourced module and the sources compiled from it.
type LocalModule struct {
	Path    string       `json:"path"`
	Dir     string       `json:"dir"`
	Sources []FileDigest `json:"sources"`
}

// NotBuilt is a file the build was asked for and could not make, with Go's
// own words for why. The adapter's test program for a package is the usual
// case: a flag the project builds with can forbid linking one.
type NotBuilt struct {
	Kind    string `json:"kind"`
	Package string `json:"package"`
	Reason  string `json:"reason"`
}
