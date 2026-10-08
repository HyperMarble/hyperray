// Purpose: the build record the adapter hands to the loader, as JSON, in the
// same shape as the Rust adapter's record.
// Never:   holds a value that was not read from the build or the machine.
package goadapter

// FileDigest names a file by the hash of its bytes.
type FileDigest struct {
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
}

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

// Artifact is one file the build made: which package, what kind of program,
// the source bytes compiled into it, and what Go says about how it was built.
type Artifact struct {
	Kind      string       `json:"kind"`
	Package   string       `json:"package"`
	File      FileDigest   `json:"file"`
	Sources   []FileDigest `json:"sources"`
	BuildInfo BuildInfo    `json:"build_info"`
}

// Toolchain is the go tool that built the module.
type Toolchain struct {
	Version  string     `json:"version"`
	Host     string     `json:"host"`
	Compiler FileDigest `json:"compiler"`
}

// Settings is what the user asked for and the files that pin every version.
type Settings struct {
	Requested Choice       `json:"requested"`
	LockFiles []FileDigest `json:"lock_files"`
}

// EnvVar is one go tool setting; a hidden value is nil.
type EnvVar struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

// NativeCode is a package whose build compiles C, C++ or assembly files.
type NativeCode struct {
	Package       string   `json:"package"`
	CgoFiles      []string `json:"cgo_files"`
	CFiles        []string `json:"c_files"`
	CxxFiles      []string `json:"cxx_files"`
	AssemblyFiles []string `json:"assembly_files"`
}

// CToolchain is the C compiler and SDK cgo uses, when cgo is on.
type CToolchain struct {
	Compiler   FileDigest `json:"compiler"`
	Version    string     `json:"version"`
	SdkPath    *string    `json:"sdk_path"`
	SdkVersion *string    `json:"sdk_version"`
}

// BuildRecord is everything needed to know exactly which code was built, and how.
type BuildRecord struct {
	Language    string       `json:"language"`
	Artifacts   []Artifact   `json:"artifacts"`
	Toolchain   Toolchain    `json:"toolchain"`
	Settings    Settings     `json:"settings"`
	Environment []EnvVar     `json:"environment"`
	NativeCode  []NativeCode `json:"native_code"`
	CToolchain  *CToolchain  `json:"c_toolchain"`
	OsBuild     string       `json:"os_build"`
}

// Outcome is the adapter's answer: a build, or the reason there is none.
type Outcome struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	*BuildRecord
}
