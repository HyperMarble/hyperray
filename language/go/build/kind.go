// Purpose: names what kind of file a build made, from the buildmode Go
// stamped into it, or from the mode the build ran with when the file is
// an archive Go's own reader cannot open; and the profile Go applied.
// Never:   calls a shared library, a plugin or an archive a program.
package build

import "github.com/HyperMarble/hyperray/language/go/record"

// kindOf is the label for every mode `go help buildmode` names.
var kindOf = map[string]string{
	"default": programKind, "exe": programKind, "pie": programKind,
	"c-shared": "c shared library", "plugin": "plugin", "shared": "shared library",
	"c-archive": "c archive", "archive": "archive",
}

// archiveMode is true for the one mode whose output Go's reader cannot open.
func archiveMode(mode string) bool {
	return mode == "c-archive"
}

// stampOf is Go's own build facts read back from the file, or nothing for
// an archive, which carries no readable stamp.
func stampOf(file built) (record.BuildInfo, error) {
	if archiveMode(file.mode) {
		return record.BuildInfo{}, nil
	}
	return ReadBuildInfo(file.path)
}

// labelOf is the kind of the built file: a test program stays one; a
// program is named by the mode Go stamped, else by the mode asked for.
func labelOf(file built, info record.BuildInfo) string {
	if file.kind != programKind {
		return file.kind
	}
	mode := file.mode
	for _, setting := range info.Settings {
		if setting.Key == "-buildmode" {
			mode = setting.Value
		}
	}
	if label := kindOf[mode]; label != "" {
		return label
	}
	return programKind
}

// profileOf hashes the profile Go says it applied: the `-pgo` setting it
// stamps into a built file names the file, with "auto" already resolved to
// the default.pgo it found. No setting means no profile was applied.
func profileOf(info record.BuildInfo) (*record.FileDigest, error) {
	for _, setting := range info.Settings {
		if setting.Key != "-pgo" || setting.Value == "off" {
			continue
		}
		digest, err := record.DigestOf(setting.Value)
		if err != nil {
			return nil, err
		}
		return &digest, nil
	}
	return nil, nil
}
