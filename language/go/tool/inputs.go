// Purpose: hashes what a build pulls in from outside the project through its
// flags and environment: an alternate go.mod and its sum, the toolexec
// wrapper, the cache program, and, through overlay.go, an overlay's files.
// Never:   refuses one of them; it names what was used.
package tool

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// Inputs fills every outside input the effective flags and go env name.
func Inputs(root string, flags []string, environment []record.EnvVar, settings *record.Settings) error {
	var err error
	if path, found := FlagValue(flags, "overlay"); found {
		if settings.Overlay, err = overlayOf(root, path); err != nil {
			return err
		}
	}
	if path, found := FlagValue(flags, "modfile"); found {
		if settings.ModFile, err = modFileOf(root, path); err != nil {
			return err
		}
	}
	if wrapper, found := FlagValue(flags, "toolexec"); found {
		if settings.ToolExec, err = namedTool(root, wrapper, false); err != nil {
			return err
		}
	}
	settings.CacheProgram, err = namedTool(root, EnvValue(environment, "GOCACHEPROG"), false)
	return err
}

// modFileOf hashes the alternate go.mod and, when it exists, its .sum.
func modFileOf(root, path string) ([]record.FileDigest, error) {
	path = under(root, path)
	digest, err := record.DigestOf(path)
	if err != nil {
		return nil, err
	}
	found := []record.FileDigest{digest}
	sum := strings.TrimSuffix(path, ".mod") + ".sum"
	if _, err := os.Stat(sum); err != nil {
		return found, nil
	}
	sumDigest, err := record.DigestOf(sum)
	if err != nil {
		return nil, err
	}
	return append(found, sumDigest), nil
}

// under is path as go reads it: relative to the folder the build runs in.
func under(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
