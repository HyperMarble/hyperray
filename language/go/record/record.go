// Purpose: the build record the adapter hands to the loader, as JSON, in the
// same shape as the Rust adapter's record.
// Never:   holds a value that was not read from the build or the machine.
package record

// FileDigest names a file by the hash of its bytes.
type FileDigest struct {
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
}

// Artifact is one file the build made: which package, what kind of program,
// the source bytes compiled into it, and what Go says about how it was built.
type Artifact struct {
	Kind      string       `json:"kind"`
	Package   string       `json:"package"`
	File      FileDigest   `json:"file"`
	Sources   []FileDigest `json:"sources"`
	Profile   *FileDigest  `json:"pgo_profile,omitempty"`
	BuildInfo BuildInfo    `json:"build_info"`
}

// Toolchain is the go installation that built the module: the go command,
// every tool in its tool directory, one hash over the standard library's
// source, and the experiments switched on.
type Toolchain struct {
	Version     string       `json:"version"`
	Host        string       `json:"host"`
	Experiments string       `json:"experiments"`
	Compiler    FileDigest   `json:"compiler"`
	Tools       []FileDigest `json:"tools"`
	StdSource   string       `json:"std_source"`
}

// EnvVar is one go tool setting; a hidden value is nil.
type EnvVar struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

// BuildRecord is everything needed to know exactly which code was built, and how.
type BuildRecord struct {
	Language       string        `json:"language"`
	Artifacts      []Artifact    `json:"artifacts"`
	Toolchain      Toolchain     `json:"toolchain"`
	Settings       Settings      `json:"settings"`
	Environment    []EnvVar      `json:"environment"`
	NativeCode     []NativeCode  `json:"native_code"`
	CToolchain     *CToolchain   `json:"c_toolchain"`
	OsBuild        string        `json:"os_build"`
	Nested         []Nested      `json:"nested"`
	IgnoredModules []FileDigest  `json:"ignored_modules"`
	LocalModules   []LocalModule `json:"local_modules"`
	Excluded       []Excluded    `json:"excluded"`
	NotBuilt       []NotBuilt    `json:"not_built"`
}

// Outcome is the adapter's answer: a build, or the reason there is none.
type Outcome struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	*BuildRecord
}
