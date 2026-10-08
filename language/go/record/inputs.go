// Purpose: what the build was asked for, what it really ran with, the files
// that pin every version, and everything a build can pull in from outside
// the project through a flag or the environment, each by hash.
// Never:   refuses a build for using one of these; it names what was used.
package record

// Overlay is a -overlay file and every file it brings into the build.
type Overlay struct {
	File    FileDigest   `json:"file"`
	Backing []FileDigest `json:"backing"`
}

// Settings is what the user asked for, what the build really ran with, and
// the inputs behind it. Tags come from Go's own stamp, since a later -tags
// replaces an earlier one. BuildFlags are every flag in the order Go reads
// them, GOFLAGS first; under -trimpath Go leaves -ldflags and the cgo flags
// out of its stamp, so this is where they survive. LockFiles are hashed
// before the build; when -mod=mod let Go change them, LockFilesAfter holds
// the new hashes and PinsChanged says so.
type Settings struct {
	Requested      Choice       `json:"requested"`
	Tags           []string     `json:"tags"`
	BuildFlags     []string     `json:"build_flags"`
	LockFiles      []FileDigest `json:"lock_files"`
	LockFilesAfter []FileDigest `json:"lock_files_after,omitempty"`
	PinsChanged    bool         `json:"pins_changed"`
	Overlay        *Overlay     `json:"overlay,omitempty"`
	ModFile        []FileDigest `json:"mod_file,omitempty"`
	ToolExec       *Tool        `json:"tool_exec,omitempty"`
	CacheProgram   *Tool        `json:"cache_program,omitempty"`
}
