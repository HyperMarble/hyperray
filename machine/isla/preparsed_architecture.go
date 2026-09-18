// A model that has already been parsed. Isla accepts either the text form or
// this one, and the text form is parsed again by every process that reads it.
package isla

import (
	"os"
	"path/filepath"
	"strings"
)

// PreparsedArchitecture returns the pre-parsed form of a model when one sits
// beside it, or the given path when none does.
//
// The pre-parsed file is named by appending .irx, which is what
// isla-preprocess writes.
func PreparsedArchitecture(path string) string {
	if strings.HasSuffix(path, ".irx") {
		return path
	}
	candidate := path + ".irx"
	if !readable(candidate) {
		return path
	}
	return candidate
}

func readable(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Size() > 0
}
